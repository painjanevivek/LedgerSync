package integration_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/transfers"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/identifier"
	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/domain/money"
	oracle "github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/tests/model"
)

type capacityReceipt struct {
	ID        string
	Digest    [32]byte
	Originals int
	Expected  string
}
type capacityWorkload struct {
	fixture  *capacityFixture
	mu       sync.Mutex
	ledger   *oracle.Ledger
	receipts map[int64]capacityReceipt
	next     atomic.Int64
}
type capacityTask struct {
	Sequence  int64
	Scheduled time.Time
}
type capacityStage struct {
	Name                   string                `json:"name"`
	Mode                   string                `json:"mode"`
	Rate                   int                   `json:"offered_per_second"`
	RequestedSeconds       int                   `json:"requested_seconds"`
	ElapsedSeconds         float64               `json:"elapsed_seconds"`
	Offered                int64                 `json:"offered"`
	Submitted              int64                 `json:"submitted"`
	Dropped                int64                 `json:"dropped"`
	OriginalPosted         int64                 `json:"new_posted"`
	Replays                int64                 `json:"replays"`
	ExpectedRejections     int64                 `json:"expected_business_rejections"`
	TechnicalFailures      int64                 `json:"technical_failures"`
	Reads                  int64                 `json:"authoritative_reads"`
	P50                    float64               `json:"command_p50_ms"`
	P95                    float64               `json:"command_p95_ms"`
	P99                    float64               `json:"command_p99_ms"`
	EndToEndP95            float64               `json:"scheduled_to_result_p95_ms"`
	ReadP95                float64               `json:"read_p95_ms"`
	PostedPerSecond        float64               `json:"new_posted_per_second"`
	PoolWaitCount          int64                 `json:"pool_wait_count"`
	PoolWaitMilliseconds   float64               `json:"pool_wait_ms"`
	ProcessCPUCorePercent  float64               `json:"driver_process_cpu_core_percent"`
	HeapAllocated          uint64                `json:"driver_heap_bytes"`
	SampledPeakPending     int64                 `json:"sampled_peak_pending"`
	SampledPeakOldest      float64               `json:"sampled_peak_oldest_seconds"`
	SampledPeakConnections int                   `json:"sampled_peak_connections"`
	SampledPeakLockWaiters int                   `json:"sampled_peak_lock_waiters"`
	Accepted               bool                  `json:"within_selected_budget"`
	StopConstraint         string                `json:"stop_constraint,omitempty"`
	Shapes                 map[string]int64      `json:"shapes"`
	Samples                []capacityQueueSample `json:"samples"`
	LatencyMilliseconds    []float64             `json:"command_latency_ms"`
}

func capacityPercentile(values []float64, quantile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copy := append([]float64(nil), values...)
	sort.Float64s(copy)
	position := int(math.Ceil(float64(len(copy))*quantile)) - 1
	if position < 0 {
		position = 0
	}
	return copy[position]
}
func newCapacityWorkload(fixture *capacityFixture) *capacityWorkload {
	accounts := map[string]oracle.Account{}
	for id, minor := range fixture.initial {
		accounts[id] = oracle.Account{Minor: strconv.FormatInt(minor, 10)}
	}
	return &capacityWorkload{fixture: fixture, ledger: oracle.New(accounts), receipts: map[int64]capacityReceipt{}}
}

func (w *capacityWorkload) execute(ctx context.Context, task capacityTask) (latency, endToEnd, readLatency float64, shape string, originalPosted, replay, rejected, read bool, err error) {
	sequence := task.Sequence
	keyIndex := sequence
	if sequence > 0 && sequence%10 == 0 {
		keyIndex = sequence - 1
	}
	pairIndex, profileErr := capacityPairIndex(w.fixture.profile, keyIndex)
	if profileErr != nil {
		err = profileErr
		return
	}
	pair := w.fixture.pairs[pairIndex]
	shape = pair.Shape
	minor := int64(1)
	expectedStatus := "posted"
	if keyIndex%50 == 49 {
		minor = 1_000_000_000
		expectedStatus = "rejected"
	}
	amount, _ := money.New("USD", minor)
	key := fmt.Sprintf("capacity-intent-%08d", keyIndex)
	command := transfers.Command{TenantID: identifier.UUID(pair.Tenant), ActorSubjectID: testActorID, DebitAccountID: identifier.UUID(pair.Source), CreditAccountID: identifier.UUID(pair.Destination), Amount: amount, IdempotencyKey: key}
	started := time.Now()
	result, submitErr := w.fixture.service.Submit(ctx, command)
	latency = float64(time.Since(started)) / float64(time.Millisecond)
	endToEnd = float64(time.Since(task.Scheduled)) / float64(time.Millisecond)
	if submitErr != nil {
		err = submitErr
		return
	}
	if result.Result.Status != expectedStatus {
		err = fmt.Errorf("unexpected capacity financial status %s", result.Result.Status)
		return
	}
	encoded, _ := json.Marshal(result.Result)
	digest := sha256.Sum256(encoded)
	w.mu.Lock()
	known, exists := w.receipts[keyIndex]
	if exists && (known.ID != result.Result.TransferID || known.Digest != digest) {
		w.mu.Unlock()
		err = fmt.Errorf("capacity replay changed immutable result")
		return
	}
	if !exists {
		outcome := w.ledger.Move(oracle.Intent{Tenant: pair.Tenant, Actor: testActorID, Operation: "transfer", Key: key, From: pair.Source, To: pair.Destination, Currency: "USD", Minor: strconv.FormatInt(minor, 10)})
		if outcome.Status != expectedStatus {
			w.mu.Unlock()
			err = fmt.Errorf("independent capacity oracle disagreed")
			return
		}
		known = capacityReceipt{ID: result.Result.TransferID, Digest: digest, Expected: expectedStatus}
	}
	if !result.Replayed {
		known.Originals++
	}
	w.receipts[keyIndex] = known
	w.mu.Unlock()
	replay = result.Replayed
	rejected = expectedStatus == "rejected"
	originalPosted = expectedStatus == "posted" && !result.Replayed
	if sequence%5 == 0 {
		readStarted := time.Now()
		balance, readErr := w.fixture.balances.ReadCurrent(ctx, pair.Tenant, testActorID, pair.Source)
		readLatency = float64(time.Since(readStarted)) / float64(time.Millisecond)
		read = true
		if readErr != nil {
			err = readErr
			return
		}
		if balance.Version < result.Result.MinimumBalanceVersions[pair.Source] {
			err = fmt.Errorf("authoritative capacity read regressed required version")
		}
	}
	return
}

func (w *capacityWorkload) stage(ctx context.Context, name, mode string, rate, seconds int, recovery bool) capacityStage {
	report := capacityStage{Name: name, Mode: mode, Rate: rate, RequestedSeconds: seconds, Accepted: true, Shapes: map[string]int64{}}
	var mu sync.Mutex
	var reads, endToEnd []float64
	stop := make(chan struct{})
	var stopOnce sync.Once
	stopWith := func(reason string) {
		mu.Lock()
		if report.StopConstraint == "" {
			report.StopConstraint = reason
		}
		report.Accepted = false
		mu.Unlock()
		stopOnce.Do(func() { close(stop) })
	}
	stageCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan capacityTask, 32)
	if mode == "closed-clients" {
		jobs = make(chan capacityTask) // Eight callers; no hidden queued demand.
	}
	var workers sync.WaitGroup
	before := w.fixture.api.Stats()
	cpuBefore, _ := capacityCPUSeconds()
	started := time.Now()
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for task := range jobs {
				callCtx, callCancel := context.WithTimeout(stageCtx, 5*time.Second)
				latency, total, readLatency, shape, posted, replay, rejected, read, err := w.execute(callCtx, task)
				callCancel()
				mu.Lock()
				report.Submitted++
				report.LatencyMilliseconds = append(report.LatencyMilliseconds, latency)
				endToEnd = append(endToEnd, total)
				report.Shapes[shape]++
				if posted {
					report.OriginalPosted++
				}
				if replay {
					report.Replays++
				}
				if rejected {
					report.ExpectedRejections++
				}
				if read {
					report.Reads++
					reads = append(reads, readLatency)
				}
				if err != nil {
					report.TechnicalFailures++
				}
				mu.Unlock()
				if err != nil {
					stopWith("technical_or_financial_error")
				}
			}
		}()
	}
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stageCtx.Done():
				return
			case <-ticker.C:
				sample, err := w.fixture.sample(stageCtx)
				if err != nil {
					if stageCtx.Err() == nil {
						stopWith("measurement_unavailable")
					}
					continue
				}
				mu.Lock()
				report.Samples = append(report.Samples, sample)
				if sample.Pending > report.SampledPeakPending {
					report.SampledPeakPending = sample.Pending
				}
				if sample.OldestSeconds > report.SampledPeakOldest {
					report.SampledPeakOldest = sample.OldestSeconds
				}
				if sample.Connections > report.SampledPeakConnections {
					report.SampledPeakConnections = sample.Connections
				}
				if sample.LockWaiters > report.SampledPeakLockWaiters {
					report.SampledPeakLockWaiters = sample.LockWaiters
				}
				mu.Unlock()
				maximumAge := 5.0
				if recovery {
					maximumAge = 60
				}
				if sample.OldestSeconds > maximumAge {
					stopWith("delivery_queue_age")
				}
				if sample.Pending > 1000 {
					stopWith("delivery_queue_depth")
				}
				if sample.Dead > 0 {
					stopWith("dead_delivery_work")
				}
				if sample.DatabaseBytes > 256*1024*1024 {
					stopWith("database_size_budget")
				}
			}
		}
	}()
	if mode == "closed-clients" {
		// Keep the old finite burst and support a separately labelled timed profile.
		var deadline <-chan time.Time
		var timer *time.Timer
		if seconds > 0 {
			timer = time.NewTimer(time.Duration(seconds) * time.Second)
			defer timer.Stop()
			deadline = timer.C
		}
	closedLoop:
		for i := 0; seconds > 0 || i < 48; i++ {
			if w.next.Load() >= 11000 {
				stopWith("operation_budget")
				break
			}
			select {
			case jobs <- capacityTask{Sequence: w.next.Add(1) - 1, Scheduled: time.Now()}:
				mu.Lock()
				report.Offered++
				mu.Unlock()
			case <-stop:
				break closedLoop
			case <-ctx.Done():
				break closedLoop
			case <-deadline:
				break closedLoop
			}
		}
	} else {
		duration := time.Duration(seconds) * time.Second
		interval := time.Second / time.Duration(rate)
	loop:
		for i := 0; ; i++ {
			scheduled := started.Add(time.Duration(i) * interval)
			if scheduled.Sub(started) >= duration {
				break
			}
			if wait := time.Until(scheduled); wait > 0 {
				timer := time.NewTimer(wait)
				select {
				case <-timer.C:
				case <-stop:
					timer.Stop()
					break loop
				case <-ctx.Done():
					timer.Stop()
					break loop
				}
			}
			mu.Lock()
			report.Offered++
			mu.Unlock()
			task := capacityTask{Sequence: w.next.Add(1) - 1, Scheduled: scheduled}
			select {
			case jobs <- task:
			default:
				mu.Lock()
				report.Dropped++
				mu.Unlock()
			}
		}
	}
	close(jobs)
	workers.Wait()
	cancel()
	<-monitorDone
	elapsed := time.Since(started)
	after := w.fixture.api.Stats()
	cpuAfter, _ := capacityCPUSeconds()
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	report.ElapsedSeconds = elapsed.Seconds()
	report.P50 = capacityPercentile(report.LatencyMilliseconds, .5)
	report.P95 = capacityPercentile(report.LatencyMilliseconds, .95)
	report.P99 = capacityPercentile(report.LatencyMilliseconds, .99)
	report.EndToEndP95 = capacityPercentile(endToEnd, .95)
	report.ReadP95 = capacityPercentile(reads, .95)
	report.PostedPerSecond = float64(report.OriginalPosted) / elapsed.Seconds()
	report.PoolWaitCount = after.WaitCount - before.WaitCount
	report.PoolWaitMilliseconds = float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)
	report.ProcessCPUCorePercent = (cpuAfter - cpuBefore) / elapsed.Seconds() * 100
	report.HeapAllocated = memory.Alloc
	if report.Dropped > 0 {
		report.Accepted = false
		if report.StopConstraint == "" {
			report.StopConstraint = "generator_saturation"
		}
	}
	if report.P95 > 500 {
		report.Accepted = false
		if report.StopConstraint == "" {
			report.StopConstraint = "command_p95_budget"
		}
	}
	return report
}
