-- Create access_requests table for escrow-style payment flow
CREATE TABLE IF NOT EXISTS access_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_address TEXT NOT NULL,
    requester_address TEXT NOT NULL,
    dataset_id BIGINT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'denied', 'paid')),
    message TEXT,
    price_apt DECIMAL DEFAULT 0.1,
    payment_tx_hash TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    approved_at TIMESTAMP,
    paid_at TIMESTAMP,
    UNIQUE(owner_address, requester_address, dataset_id)
);

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_access_requests_owner ON access_requests(owner_address);
CREATE INDEX IF NOT EXISTS idx_access_requests_requester ON access_requests(requester_address);
CREATE INDEX IF NOT EXISTS idx_access_requests_status ON access_requests(status);
CREATE INDEX IF NOT EXISTS idx_access_requests_created ON access_requests(created_at DESC);

-- Enable Row Level Security (optional, for added security)
ALTER TABLE access_requests ENABLE ROW LEVEL SECURITY;

-- Allow anyone to insert (since we're using wallet-based auth, not Supabase Auth)
CREATE POLICY access_requests_insert_policy ON access_requests
    FOR INSERT
    WITH CHECK (true);

-- Allow anyone to read
CREATE POLICY access_requests_select_policy ON access_requests
    FOR SELECT
    USING (true);

-- Allow anyone to update (we validate ownership in application logic)
CREATE POLICY access_requests_update_policy ON access_requests
    FOR UPDATE
    USING (true);

-- Alternative: Disable RLS completely for simpler setup
-- ALTER TABLE access_requests DISABLE ROW LEVEL SECURITY;
