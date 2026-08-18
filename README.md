# FinTech Distributed System

This project demonstrates a read-your-writes consistency pattern in a distributed financial system. It ensures that when a user transfers money, they immediately see their updated balance, even when reading from replicas or caches that might have stale data.

## Architecture Overview

The system consists of several interconnected services:

1. **Account Service** (`backend/account_service`) - Handles money transfers and balance queries using PostgreSQL
2. **Cache Service** (`backend/cache_service`) - Manages Redis caching of account balances
3. **Event Bus Service** (`backend/event_bus`) - Listens to Redis Streams for balance invalidation events
4. **Auth Service** (`backend/auth`) - Generates and validates JWT tokens for read-your-writes consistency
5. **Simulation Service** (`simulation`) - Python-based dashboard to simulate network lag, stale reads, and invalidation delays
6. **Frontend Dashboard** (`dashboard`) - Next.js interface for users to interact with the system

## Key Concept: Read-Your-Writes (RYEW) Tokens

When a user performs a money transfer:
1. The transfer is written to the primary PostgreSQL database
2. Cache entries for the affected accounts are invalidated via Redis Streams
3. The Account Service generates a RYEW token (JWT) and returns it to the user
4. On subsequent balance requests, the user includes this token
5. The system checks if any invalidation events occurred after the token was issued
6. If so, it waits for cache to be refreshed before returning the balance
7. This ensures the user always sees their post-transfer balance

## Prerequisites

- Docker and Docker Compose
- Node.js 18+ (for frontend development)
- Go 1.22+ (for backend services - optional if using Docker)
- Python 3.11+ (for simulation service - optional if using Docker)

## Running the System

### Option 1: Using Docker Compose (Recommended)

```bash
docker-compose up --build
```

This will start all services:
- PostgreSQL Primary (port 5432)
- PostgreSQL Replica (port 5433)
- Redis (port 6379)
- Account Service (gRPC on port 50051)
- Cache Service (port 50052)
- Event Bus Service (port 50053)
- Auth Service (port 50054)
- Simulation Service (HTTP on port 8000)
- Frontend Dashboard (HTTP on port 3000)

### Option 2: Running Services Individually

#### Backend Services (Go)
```bash
# Account Service
cd backend/account_service
go run main.go

# Cache Service
cd backend/cache_service
go run main.go

# Event Bus Service
cd backend/event_bus
go run main.go

# Auth Service
cd backend/auth
go run main.go
```

#### Simulation Service (Python)
```bash
cd simulation
pip install -r requirements.txt
python sim_service.py
```

#### Frontend Dashboard (Next.js)
```bash
cd dashboard
npm install
npm run dev
```

## Testing the System

1. Open your browser to `http://localhost:3000`
2. Select an account from the dropdown to see its balance
3. Use the transfer form to send money between accounts
4. Observe that your balance updates immediately after transfer
5. Use the Simulation Controls tab to introduce lag, stale reads, or invalidation delays
6. Notice how the RYEW token mechanism prevents you from seeing stale balances

## Simulation Controls

The simulation service (`http://localhost:8000`) allows you to test various network conditions:

- **Lag Simulation**: Delays cache updates to simulate slow replicas
- **Stale Read Simulation**: Randomly returns cached (potentially stale) data
- **Invalidation Delay**: Delays propagation of invalidation events through the system
- **Manual Invalidation**: Manually trigger cache invalidation for specific accounts

## Project Structure

```
.
├── docker-compose.yml
├── README.md
├── backend/
│   ├── account_service/
│   │   ├── main.go
│   │   ├── Dockerfile
│   │   └── go.mod
│   ├── auth/
│   │   ├── main.go
│   │   ├── Dockerfile
│   │   └── go.mod
│   ├── cache_service/
│   │   ├── main.go
│   │   ├── Dockerfile
│   │   └── go.mod
│   ├── event_bus/
│   │   ├── main.go
│   │   ├── Dockerfile
│   │   └── go.mod
│   └── proto/
│       └── balance.proto
├── simulation/
│   ├── sim_service.py
│   ├── Dockerfile
│   └── requirements.txt
├── dashboard/
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   └── styles/
│   ├── package.json
│   └── next.config.js
├── setup/
│   └── postgres-setup.sql
└── tests/
    ├── consistency_test.go
    └── dashboard_test.py
```

## How It Works

1. **Money Transfer**:
   - User initiates transfer via frontend
   - Account Service debits source account and credits destination account in PostgreSQL (transaction)
   - Account Service publishes invalidation events to Redis Stream for both accounts
   - Account Service generates and returns RYEW token (JWT) containing affected account IDs

2. **Balance Query**:
   - User requests balance with RYEW token
   - System checks if token-associated accounts have been invalidated since token issuance
   - If invalidated, wait for cache refresh or fetch directly from database
   - Return balance with staleness flag if appropriate

3. **Cache Management**:
   - Cache Service listens for invalidation events and removes stale entries
   - Balance queries populate cache on cache misses
   - Event Bus Service processes invalidation events for monitoring/alerting

## Future Improvements

1. Implement actual PostgreSQL replication between primary and replica
2. Add proper error handling and retry mechanisms
3. Add metrics collection and monitoring (Prometheus/Grafana)
4. Implement token blacklisting for security
5. Add unit and integration tests
6. Implement persistent storage for RYEW tokens
7. Add authentication and authorization layers
8. Implement horizontal scaling considerations

## License

MIT