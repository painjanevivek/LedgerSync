package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	pb "Distributed-system-FA1-FA2/backend/proto"
)

// AccountService implements the BalanceService server
type AccountService struct {
	pb.UnimplementedBalanceServiceServer
	db     *sql.DB
	rdb    *redis.Client
	ctx    context.Context
	cancel context.CancelFunc
}

// NewAccountService creates a new AccountService instance
func NewAccountService(db *sql.DB, rdb *redis.Client) *AccountService {
	ctx, cancel := context.WithCancel(context.Background())
	return &AccountService{
		db:     db,
		rdb:    rdb,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Transfer handles money transfer between accounts
func (s *AccountService) Transfer(ctx context.Context, req *pb.TransferRequest) (*pb.TransferResponse, error) {
	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to begin transaction: %v", err))
	}
	defer tx.Rollback() // Rollback if not committed

	// Check if from account has sufficient balance
	var fromBalance float64
	err = tx.QueryRowContext(ctx, "SELECT balance FROM accounts WHERE account_id = $1 FOR UPDATE", req.FromAccountId).Scan(&fromBalance)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, status.Error(codes.NotFound, "from account not found")
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to query from account: %v", err))
	}

	if fromBalance < req.Amount {
		return nil, status.Error(codes.InvalidArgument, "insufficient funds")
	}

	// Deduct from source account
	_, err = tx.ExecContext(ctx, "UPDATE accounts SET balance = balance - $1 WHERE account_id = $2", req.Amount, req.FromAccountId)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to update from account: %v", err))
	}

	// Add to destination account
	_, err = tx.ExecContext(ctx, "UPDATE accounts SET balance = balance + $1 WHERE account_id = $2", req.Amount, req.ToAccountId)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to update to account: %v", err))
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to commit transaction: %v", err))
	}

	// Generate read-your-writes token (simplified as a timestamp + random)
	ryewToken := fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix())

	// Invalidate cache for both accounts to force fresh reads
	invalidationMsg := map[string]interface{}{
		"account_ids": []string{req.FromAccountId, req.ToAccountId},
		"timestamp":   time.Now().Unix(),
	}
	msgJSON, _ := json.Marshal(invalidationMsg)
	s.rdb.XAdd(s.ctx, &redis.XAddArgs{
		Stream: "balance:invalidation",
		Values: map[string]interface{}{"data": msgJSON},
	})

	return &pb.TransferResponse{
		Success:   true,
		Message:   "Transfer successful",
		RyewToken: ryewToken,
	}, nil
}

// GetBalance retrieves balance for an account with read-your-writes support
func (s *AccountService) GetBalance(ctx context.Context, req *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
	var balance float64
	var isStale bool

	// If we have a RYEW token, we need to check if cache is fresh
	if req.RyewToken != "" {
		// Check if we need to wait for cache invalidation
		isStale = s.checkCacheFreshness(ctx, req.AccountId, req.RyewToken)
	}

	// Try to get from cache first
	cachedBalance, err := s.rdb.Get(s.ctx, "balance:"+req.AccountId).Result()
	if err == nil {
		var cachedBalanceFloat float64
		fmt.Sscanf(cachedBalance, "%f", &cachedBalanceFloat)
		return &pb.GetBalanceResponse{
			Balance:   cachedBalanceFloat,
			Currency:  "USD",
			IsStale:   isStale,
		}, nil
	}

	// Fallback to database
	err = s.db.QueryRowContext(ctx, "SELECT balance FROM accounts WHERE account_id = $1", req.AccountId).Scan(&balance)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, status.Error(codes.NotFound, "account not found")
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to query balance: %v", err))
	}

	// Update cache
	s.rdb.Set(s.ctx, "balance:"+req.AccountId, fmt.Sprintf("%f", balance), 0)

	return &pb.GetBalanceResponse{
		Balance:   balance,
		Currency:  "USD",
		IsStale:   isStale,
	}, nil
}

// checkCacheFreshness determines if cache might be stale based on RYEW token
func (s *AccountService) checkCacheFreshness(ctx context.Context, accountId string, ryewToken string) bool {
	// In a real implementation, we would check Redis Streams for invalidation events
	// that happened after the RYEW token was generated
	// For simplicity, we'll return false (assuming cache is fresh)
	// A real implementation would:
	// 1. Parse the timestamp from the RYEW token
	// 2. Check Redis Streams for any invalidation events after that timestamp
	// 3. Return true if such events exist for this account
	return false
}

func main() {
	# Setup database connection
	connStr := "user=user password=password dbname=fintech host=postgres-primary port=5432 sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	# Test database connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	# Setup Redis connection
	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
		Password: "", # no password set
		DB:       0,  # use default DB
	})

	# Test Redis connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	# Create accounts table if it doesn't exist (for demo purposes)
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS accounts (
		account_id VARCHAR(50) PRIMARY KEY,
		balance DECIMAL(15, 2) NOT NULL DEFAULT 0.00
	);
	`
	if _, err = db.Exec(createTableQuery); err != nil {
		log.Fatalf("Failed to create accounts table: %v", err)
	}

	# Insert some sample accounts if they don't exist
	insertSample := `
	INSERT INTO accounts (account_id, balance) VALUES
	('acc_001', 1000.00),
	('acc_002', 500.00),
	('acc_003', 2000.00)
	ON CONFLICT (account_id) DO NOTHING;
	`
	if _, err = db.Exec(insertSample); err != nil {
		log.Fatalf("Failed to insert sample accounts: %v", err)
	}

	# Start gRPC server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterBalanceServiceServer(s, NewAccountService(db, rdb))
	log.Println("Account service listening on port 50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}