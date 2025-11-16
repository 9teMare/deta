package models

// Request models
type InitializeUserRequest struct {
	AccountAddress string `json:"account_address" binding:"required"`
}

type SubmitDataRequest struct {
	PrivateKey string `json:"private_key" binding:"required"`
	DataHash   string `json:"data_hash" binding:"required"`
	Metadata   string `json:"metadata"`
}

type DeleteDatasetRequest struct {
	PrivateKey string `json:"private_key" binding:"required"`
	DatasetID  uint64 `json:"dataset_id" binding:"required"`
}

type GrantAccessRequest struct {
	PrivateKey string `json:"private_key" binding:"required"`
	DatasetID  uint64 `json:"dataset_id" binding:"required"`
	Requester  string `json:"requester" binding:"required"`
	ExpiresAt  uint64 `json:"expires_at" binding:"required"`
}

type RevokeAccessRequest struct {
	PrivateKey string `json:"private_key" binding:"required"`
	DatasetID  uint64 `json:"dataset_id" binding:"required"`
	Requester  string `json:"requester" binding:"required"`
}

type CheckAccessRequest struct {
	Owner     string `json:"owner" binding:"required"`
	DatasetID uint64 `json:"dataset_id" binding:"required"`
	Requester string `json:"requester" binding:"required"`
}

type RegisterTokenRequest struct {
	PrivateKey string `json:"private_key" binding:"required"`
}

type MintTokenRequest struct {
	PrivateKey string `json:"private_key" binding:"required"`
	Recipient  string `json:"recipient" binding:"required"`
	Amount     uint64 `json:"amount" binding:"required"`
}

type GetDatasetRequest struct {
	User      string `json:"user" binding:"required"`
	DatasetID uint64 `json:"dataset_id" binding:"required"`
}

type GetUserVaultRequest struct {
	User string `json:"user" binding:"required"`
}

type CheckInitializationRequest struct {
	User string `json:"user" binding:"required"`
}

// Response models
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type TransactionResponse struct {
	Hash    string `json:"hash"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type DatasetInfo struct {
	ID        uint64 `json:"id"`
	Owner     string `json:"owner"`
	DataHash  string `json:"data_hash"`
	Metadata  string `json:"metadata"`
	CreatedAt uint64 `json:"created_at"`
	IsActive  bool   `json:"is_active"`
}

type AccessInfo struct {
	HasAccess bool   `json:"has_access"`
	ExpiresAt uint64 `json:"expires_at,omitempty"`
}

type VaultInfo struct {
	Datasets []uint64 `json:"datasets"`
	Count    uint64   `json:"count"`
}

type InitializationInfo struct {
	Initialized bool `json:"initialized"`
}

type SubmitCSVRequest struct {
	AccountAddress string `json:"account_address" binding:"required"`
	DataHash       string `json:"data_hash" binding:"required"`
	Schema         string `json:"schema" binding:"required"`
	CSVData        string `json:"csv_data" binding:"required"`
}
