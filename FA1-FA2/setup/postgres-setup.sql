-- Create accounts table
CREATE TABLE IF NOT EXISTS accounts (
    account_id VARCHAR(50) PRIMARY KEY,
    balance DECIMAL(15, 2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert sample accounts
INSERT INTO accounts (account_id, balance) VALUES
('acc_001', 1000.00),
('acc_002', 500.00),
('acc_003', 2000.00),
('acc_004', 1500.00),
('acc_005', 3000.00)
ON CONFLICT (account_id) DO NOTHING;

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_accounts_balance ON accounts(balance);