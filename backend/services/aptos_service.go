package services

// This file defines the interface for AptosService
// The implementation is in aptos_service_impl.go

type AptosService interface {
	InitializeUser(privateKeyHex string) (string, error)
	SubmitData(privateKeyHex string, dataHash string, metadata string) (string, error)
	DeleteDataset(privateKeyHex string, datasetID uint64) (string, error)
	GrantAccess(privateKeyHex string, datasetID uint64, requester string, expiresAt uint64) (string, error)
	RevokeAccess(privateKeyHex string, datasetID uint64, requester string) (string, error)
	RegisterToken(privateKeyHex string) (string, error)
	MintToken(privateKeyHex string, recipient string, amount uint64) (string, error)
	GetDataset(userAddress string, datasetID uint64) (interface{}, error)
	CheckAccess(owner string, datasetID uint64, requester string) (bool, error)
	GetUserVault(userAddress string) ([]uint64, error)
	GetUserDatasetsMetadata(userAddress string) ([]interface{}, error) // Returns minimal metadata (id, metadata, is_active) for all datasets
	IsAccountInitialized(userAddress string) (bool, error)
	GetMarketplaceDatasets() ([]interface{}, error)
	GetAccessRequests(ownerAddress string) ([]interface{}, error)
	CheckDataHashExists(dataHash string) (bool, error)
}
