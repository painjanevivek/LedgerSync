package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers/internal/application/investigation"
	"github.com/redis/go-redis/v9"
)

const livePresenceTTL = 20 * time.Second

var redisStreamCursor = regexp.MustCompile(`^(?:0-0|[1-9][0-9]*-[0-9]+)$`)

type LiveInvestigationSignals struct {
	client    redis.UniversalClient
	namespace string
}

func NewLiveInvestigationSignals(client redis.UniversalClient, namespace string) (*LiveInvestigationSignals, error) {
	namespace = strings.TrimSpace(namespace)
	if client == nil || namespace == "" || len(namespace) > 80 || strings.ContainsAny(namespace, " \t\r\n*?[]") {
		return nil, errors.New("valid redis client and live investigation namespace are required")
	}
	return &LiveInvestigationSignals{client: client, namespace: namespace}, nil
}

var appendLiveSignal = redis.NewScript(`
local previous = redis.call('GET', KEYS[2])
if previous and tonumber(ARGV[2]) <= tonumber(previous) then
  return redis.error_reply('sequence_not_increasing')
end
redis.call('SET', KEYS[2], ARGV[2], 'PX', ARGV[6])
local id = redis.call('XADD', KEYS[1], 'MAXLEN', ARGV[5], '*',
  'role', ARGV[1], 'sequence', ARGV[2], 'kind', ARGV[3], 'payload', ARGV[4], 'sent_at', ARGV[7])
redis.call('PEXPIRE', KEYS[1], ARGV[6])
return id
`)

func (s *LiveInvestigationSignals) Append(ctx context.Context, signal investigation.LiveSignal) error {
	signal, err := investigation.NormalizeLiveSignal(signal)
	if err != nil {
		return err
	}
	when := signal.SentAt.UTC()
	if when.IsZero() {
		when = time.Now().UTC()
	}
	_, err = appendLiveSignal.Run(ctx, s.client,
		[]string{s.streamKey(signal.RoomID), s.sequenceKey(signal.RoomID, signal.Role)},
		signal.Role, signal.Sequence, signal.Kind, string(signal.Payload), investigation.MaxLiveSignals,
		investigation.LiveSignalTTL.Milliseconds(), when.Format(time.RFC3339Nano),
	).Result()
	if err != nil {
		if strings.Contains(err.Error(), "sequence_not_increasing") {
			return investigation.ErrLiveRoomConflict
		}
		return fmt.Errorf("append live investigation signal: %w", err)
	}
	return nil
}

func (s *LiveInvestigationSignals) Read(ctx context.Context, roomID, cursor string) (investigation.LiveSignalPage, error) {
	roomID, err := investigation.NormalizeLiveRoomID(roomID)
	if err != nil {
		return investigation.LiveSignalPage{}, investigation.ErrLiveRoomNotFound
	}
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		cursor = "0-0"
	}
	if !redisStreamCursor.MatchString(cursor) {
		return investigation.LiveSignalPage{}, investigation.ErrInvalidLiveRoom
	}
	messages, err := s.client.XRangeN(ctx, s.streamKey(roomID), "("+cursor, "+", investigation.MaxLiveSignals).Result()
	if errors.Is(err, redis.Nil) {
		messages, err = nil, nil
	}
	if err != nil {
		return investigation.LiveSignalPage{}, fmt.Errorf("read live investigation signals: %w", err)
	}
	page := investigation.LiveSignalPage{Signals: make([]investigation.LiveSignal, 0, len(messages)), Cursor: cursor}
	for _, message := range messages {
		signal, parseErr := decodeLiveSignal(roomID, message)
		if parseErr != nil {
			return investigation.LiveSignalPage{}, parseErr
		}
		page.Signals = append(page.Signals, signal)
		page.Cursor = message.ID
	}
	return page, nil
}

func (s *LiveInvestigationSignals) Presence(ctx context.Context, roomID, role string) error {
	roomID, err := investigation.NormalizeLiveRoomID(roomID)
	if err != nil || role != "owner" && role != "peer" {
		return investigation.ErrInvalidLiveRoom
	}
	if err := s.client.Set(ctx, s.presenceKey(roomID, role), "1", livePresenceTTL).Err(); err != nil {
		return fmt.Errorf("refresh live investigation presence: %w", err)
	}
	return nil
}

func (s *LiveInvestigationSignals) IsPresent(ctx context.Context, roomID, role string) (bool, error) {
	roomID, err := investigation.NormalizeLiveRoomID(roomID)
	if err != nil || role != "owner" && role != "peer" {
		return false, investigation.ErrInvalidLiveRoom
	}
	exists, err := s.client.Exists(ctx, s.presenceKey(roomID, role)).Result()
	if err != nil {
		return false, fmt.Errorf("read live investigation presence: %w", err)
	}
	return exists == 1, nil
}

func (s *LiveInvestigationSignals) Clear(ctx context.Context, roomID string) error {
	roomID, err := investigation.NormalizeLiveRoomID(roomID)
	if err != nil {
		return investigation.ErrLiveRoomNotFound
	}
	keys := []string{s.streamKey(roomID), s.sequenceKey(roomID, "owner"), s.sequenceKey(roomID, "peer"), s.presenceKey(roomID, "owner"), s.presenceKey(roomID, "peer")}
	if err := s.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("clear live investigation signals: %w", err)
	}
	return nil
}

func decodeLiveSignal(roomID string, message redis.XMessage) (investigation.LiveSignal, error) {
	stringValue := func(name string) string {
		value, ok := message.Values[name]
		if !ok {
			return ""
		}
		return fmt.Sprint(value)
	}
	sequence, err := strconv.ParseInt(stringValue("sequence"), 10, 64)
	if err != nil {
		return investigation.LiveSignal{}, fmt.Errorf("decode live investigation signal: %w", err)
	}
	when, err := time.Parse(time.RFC3339Nano, stringValue("sent_at"))
	if err != nil {
		return investigation.LiveSignal{}, fmt.Errorf("decode live investigation signal: %w", err)
	}
	signal, err := investigation.NormalizeLiveSignal(investigation.LiveSignal{
		ID: message.ID, RoomID: roomID, Role: stringValue("role"), Sequence: sequence,
		Kind: stringValue("kind"), Payload: json.RawMessage(stringValue("payload")), SentAt: when,
	})
	if err != nil {
		return investigation.LiveSignal{}, fmt.Errorf("decode live investigation signal: %w", err)
	}
	return signal, nil
}

func (s *LiveInvestigationSignals) streamKey(roomID string) string {
	return s.namespace + ":" + roomID + ":signals"
}
func (s *LiveInvestigationSignals) sequenceKey(roomID, role string) string {
	return s.namespace + ":" + roomID + ":sequence:" + role
}
func (s *LiveInvestigationSignals) presenceKey(roomID, role string) string {
	return s.namespace + ":" + roomID + ":presence:" + role
}

var _ investigation.LiveSignalStore = (*LiveInvestigationSignals)(nil)
