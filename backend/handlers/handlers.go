package handlers

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/datax/backend/models"
	"github.com/datax/backend/services"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	aptosService   services.AptosService
	storageService services.StorageService
}

func NewHandler(aptosService services.AptosService, storageService services.StorageService) *Handler {
	return &Handler{
		aptosService:   aptosService,
		storageService: storageService,
	}
}

// Note: All in-memory storage has been removed
// CSV data is stored in Supabase S3, and blob names are discovered via storage service

// InitializeUser - Note: Transactions are now signed on the frontend
// This endpoint is kept for backward compatibility but returns a message
func (h *Handler) InitializeUser(c *gin.Context) {
	var req models.InitializeUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Transactions are now signed on the frontend using wallet adapter
	// This endpoint just acknowledges the request
	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "User initialization should be done via wallet-signed transaction on the frontend",
		Data: models.TransactionResponse{
			Hash:    "",
			Success: true,
			Message: "Please use the frontend to initialize your account with your wallet",
		},
	})
}

// CheckDataHash checks if a data hash already exists
func (h *Handler) CheckDataHash(c *gin.Context) {
	var req struct {
		DataHash string `json:"data_hash" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	exists, err := h.aptosService.CheckDataHashExists(req.DataHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data:    exists,
	})
}

// DeleteDataset deletes a dataset
func (h *Handler) DeleteDataset(c *gin.Context) {
	var req models.DeleteDatasetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	txHash, err := h.aptosService.DeleteDataset(req.PrivateKey, req.DatasetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data: models.TransactionResponse{
			Hash:    txHash,
			Success: true,
			Message: "Dataset deleted successfully",
		},
	})
}

// GrantAccess grants access to a requester
func (h *Handler) GrantAccess(c *gin.Context) {
	var req models.GrantAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	txHash, err := h.aptosService.GrantAccess(req.PrivateKey, req.DatasetID, req.Requester, req.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data: models.TransactionResponse{
			Hash:    txHash,
			Success: true,
			Message: "Access granted successfully",
		},
	})
}

// RevokeAccess revokes access from a requester
func (h *Handler) RevokeAccess(c *gin.Context) {
	var req models.RevokeAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	txHash, err := h.aptosService.RevokeAccess(req.PrivateKey, req.DatasetID, req.Requester)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data: models.TransactionResponse{
			Hash:    txHash,
			Success: true,
			Message: "Access revoked successfully",
		},
	})
}

// CheckAccess checks if a requester has access
func (h *Handler) CheckAccess(c *gin.Context) {
	var req models.CheckAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	hasAccess, err := h.aptosService.CheckAccess(req.Owner, req.DatasetID, req.Requester)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data: models.AccessInfo{
			HasAccess: hasAccess,
		},
	})
}

// GetDataset retrieves dataset information
func (h *Handler) GetDataset(c *gin.Context) {
	// First, try to bind to a map to handle flexible types
	var rawBody map[string]interface{}
	if err := c.ShouldBindJSON(&rawBody); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   fmt.Sprintf("Invalid JSON: %v", err),
		})
		return
	}

	// Extract and validate user
	user, ok := rawBody["user"].(string)
	if !ok || user == "" {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "user is required and must be a non-empty string",
		})
		return
	}

	// Extract and convert dataset_id (handle both string and number)
	var datasetID uint64
	datasetIDVal, ok := rawBody["dataset_id"]
	if !ok {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "dataset_id is required",
		})
		return
	}

	switch v := datasetIDVal.(type) {
	case float64:
		datasetID = uint64(v)
	case string:
		parsed, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.Response{
				Success: false,
				Error:   fmt.Sprintf("dataset_id must be a valid number: %v", err),
			})
			return
		}
		datasetID = parsed
	case uint64:
		datasetID = v
	case int:
		datasetID = uint64(v)
	default:
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "dataset_id must be a number",
		})
		return
	}

	if datasetID == 0 {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "dataset_id must be greater than 0",
		})
		return
	}

	var req models.GetDatasetRequest
	req.User = user
	req.DatasetID = datasetID

	datasetRaw, err := h.aptosService.GetDataset(req.User, req.DatasetID)
	if err != nil {
		fmt.Printf("ERROR: GetDataset failed: %v\n", err)
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Convert the raw result to DatasetInfo format
	datasetMap, ok := datasetRaw.(map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   "unexpected dataset format",
		})
		return
	}

	// The service now returns data_hash as hex string and metadata as string
	dataHashHex, _ := datasetMap["data_hash"].(string)
	metadataStr, _ := datasetMap["metadata"].(string)

	var createdAt uint64
	switch v := datasetMap["created_at"].(type) {
	case float64:
		createdAt = uint64(v)
	case uint64:
		createdAt = v
	case string:
		parsed, _ := strconv.ParseUint(v, 10, 64)
		createdAt = parsed
	}

	isActive, _ := datasetMap["is_active"].(bool)

	dataset := models.DatasetInfo{
		ID:        req.DatasetID,
		Owner:     req.User,
		DataHash:  dataHashHex,
		Metadata:  metadataStr,
		CreatedAt: createdAt,
		IsActive:  isActive,
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data:    dataset,
	})
}

// GetMarketplaceDatasets retrieves all datasets from the marketplace
func (h *Handler) GetMarketplaceDatasets(c *gin.Context) {
	fmt.Printf("DEBUG: GetMarketplaceDatasets endpoint called\n")
	startTime := time.Now()

	datasets, err := h.aptosService.GetMarketplaceDatasets()
	elapsed := time.Since(startTime)

	if err != nil {
		fmt.Printf("ERROR: GetMarketplaceDatasets failed after %v: %v\n", elapsed, err)
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   fmt.Sprintf("Failed to fetch marketplace datasets: %v", err),
		})
		return
	}

	fmt.Printf("DEBUG: GetMarketplaceDatasets completed in %v, returning %d datasets\n", elapsed, len(datasets))
	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data:    datasets,
	})
}

// GetAccessRequests retrieves access requests for a dataset owner
func (h *Handler) GetAccessRequests(c *gin.Context) {
	var req struct {
		Owner string `json:"owner" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	requests, err := h.aptosService.GetAccessRequests(req.Owner)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data:    requests,
	})
}

// RequestAccess creates an access request
func (h *Handler) RequestAccess(c *gin.Context) {
	var req struct {
		Owner     string `json:"owner" binding:"required"`
		DatasetID uint64 `json:"dataset_id" binding:"required"`
		Requester string `json:"requester" binding:"required"`
		Message   string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	services.RequestAccess(req.Owner, req.DatasetID, req.Requester, req.Message)

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "Access request submitted",
	})
}

// RegisterUserForMarketplace allows users to manually register themselves
// This is useful if they submitted data before the registry was set up
func (h *Handler) RegisterUserForMarketplace(c *gin.Context) {
	var req struct {
		UserAddress string `json:"user_address" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// User discovery is now automatic from the blockchain
	// No registration needed - users are discovered by querying recent transactions
	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "User discovery is automatic from the blockchain. No registration needed.",
	})
}

// GetCSVData retrieves CSV data if user has access
func (h *Handler) GetCSVData(c *gin.Context) {
	fmt.Printf("DEBUG: GetCSVData endpoint called\n")
	fmt.Printf("DEBUG: Request method: %s, Path: %s\n", c.Request.Method, c.Request.URL.Path)

	var req struct {
		DataHash  string `json:"data_hash" binding:"required"`
		Owner     string `json:"owner" binding:"required"`
		DatasetID uint64 `json:"dataset_id" binding:"required"`
		Requester string `json:"requester" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("ERROR: Failed to bind request: %v\n", err)
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	fmt.Printf("DEBUG: GetCSVData request - dataHash=%s, owner=%s, datasetID=%d, requester=%s\n", req.DataHash, req.Owner, req.DatasetID, req.Requester)

	// Check if requester is the owner (owners can always view their data)
	isOwner := (req.Requester == req.Owner)

	var hasAccess bool
	if !isOwner {
		// Check if requester has access
		var err error
		hasAccess, err = h.aptosService.CheckAccess(req.Owner, req.DatasetID, req.Requester)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.Response{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
	} else {
		hasAccess = true
	}

	if !hasAccess {
		c.JSON(http.StatusForbidden, models.Response{
			Success: false,
			Error:   "Access denied",
		})
		return
	}

	// Retrieve CSV data directly from storage service
	// Try using the data hash directly first (in case it's already a blob name)
	// Also try if blob name contains "/" (Supabase format: {account}/{timestamp}_{hash}.csv)
	var csvData [][]string
	var err error

	if strings.HasPrefix(req.DataHash, "csv_") || strings.Contains(req.DataHash, "/") {
		fmt.Printf("DEBUG: Data hash looks like a blob name, trying direct retrieval: %s\n", req.DataHash)
		csvData, err = h.storageService.RetrieveCSV(req.Owner, req.DataHash)
		if err != nil {
			fmt.Printf("DEBUG: Direct retrieval failed, trying to find blob by pattern: %v\n", err)
		}
	} else {
		// Try direct retrieval first
		csvData, err = h.storageService.RetrieveCSV(req.Owner, req.DataHash)
		if err != nil {
			fmt.Printf("DEBUG: Direct retrieval failed, trying to find blob by pattern: %v\n", err)
		}
	}

	// If direct retrieval failed, try to find blob by listing S3 objects
	if err != nil {
		fmt.Printf("DEBUG: Attempting to find blob by listing S3 objects for owner: %s\n", req.Owner)
		if supabaseService, ok := h.storageService.(interface {
			FindBlobByPattern(accountAddress string, pattern string) (string, error)
		}); ok {
			// Try with empty pattern to list all objects for this owner and get the most recent CSV
			blobName, findErr := supabaseService.FindBlobByPattern(req.Owner, "")
			if findErr == nil {
				fmt.Printf("DEBUG: Found blob by listing: %s\n", blobName)
				csvData, err = h.storageService.RetrieveCSV(req.Owner, blobName)
				if err != nil {
					fmt.Printf("ERROR: Failed to retrieve after listing: %v\n", err)
					c.JSON(http.StatusNotFound, models.Response{
						Success: false,
						Error:   fmt.Sprintf("CSV data not found in storage: %v", err),
					})
					return
				}
				fmt.Printf("DEBUG: Successfully retrieved CSV from storage: %s\n", blobName)
			} else {
				fmt.Printf("ERROR: Listing objects failed: %v\n", findErr)
				c.JSON(http.StatusNotFound, models.Response{
					Success: false,
					Error:   fmt.Sprintf("CSV data not found. Data hash: %s. Error: %v", req.DataHash, findErr),
				})
				return
			}
		} else {
			c.JSON(http.StatusNotFound, models.Response{
				Success: false,
				Error:   fmt.Sprintf("CSV data not found. Data hash: %s. The file may not have been stored.", req.DataHash),
			})
			return
		}
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data:    csvData,
	})
}

// GetUserVault retrieves user's vault datasets
func (h *Handler) GetUserVault(c *gin.Context) {
	var req models.GetUserVaultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	datasets, err := h.aptosService.GetUserVault(req.User)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data: models.VaultInfo{
			Datasets: datasets,
			Count:    uint64(len(datasets)),
		},
	})
}

// GetUserDatasetsMetadata retrieves minimal metadata for all user datasets (optimized for batch operations)
func (h *Handler) GetUserDatasetsMetadata(c *gin.Context) {
	var req models.GetUserVaultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	metadata, err := h.aptosService.GetUserDatasetsMetadata(req.User)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data:    metadata,
	})
}

// CheckInitialization checks if the user account is initialized
func (h *Handler) CheckInitialization(c *gin.Context) {
	var req models.CheckInitializationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	initialized, err := h.aptosService.IsAccountInitialized(req.User)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data: models.InitializationInfo{
			Initialized: initialized,
		},
	})
}

// RegisterToken registers a user to receive tokens
func (h *Handler) RegisterToken(c *gin.Context) {
	var req models.RegisterTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	txHash, err := h.aptosService.RegisterToken(req.PrivateKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data: models.TransactionResponse{
			Hash:    txHash,
			Success: true,
			Message: "Token registration successful",
		},
	})
}

// MintToken mints tokens to a recipient
func (h *Handler) MintToken(c *gin.Context) {
	var req models.MintTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	txHash, err := h.aptosService.MintToken(req.PrivateKey, req.Recipient, req.Amount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Data: models.TransactionResponse{
			Hash:    txHash,
			Success: true,
			Message: "Tokens minted successfully",
		},
	})
}

// SubmitCSV handles CSV file upload and processing
func (h *Handler) SubmitCSV(c *gin.Context) {
	accountAddress := c.PostForm("account_address")
	dataHash := c.PostForm("data_hash")
	schemaJSON := c.PostForm("schema")

	if accountAddress == "" || dataHash == "" || schemaJSON == "" {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "Missing required fields: account_address, data_hash, schema",
		})
		return
	}

	// Get the uploaded CSV file
	file, err := c.FormFile("csv_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "Missing CSV file: " + err.Error(),
		})
		return
	}

	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   "Failed to open uploaded file: " + err.Error(),
		})
		return
	}
	defer src.Close()

	// Read and parse CSV file
	csvReader := csv.NewReader(src)
	csvData, err := csvReader.ReadAll()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "Failed to parse CSV file: " + err.Error(),
		})
		return
	}

	// Parse schema
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "Invalid schema JSON: " + err.Error(),
		})
		return
	}

	fmt.Printf("DEBUG: CSV submitted for user %s\n", accountAddress)

	// Store CSV data in Supabase S3
	blobName, err := h.storageService.StoreCSV(accountAddress, csvData)
	if err != nil {
		fmt.Printf("ERROR: Failed to store CSV in Supabase S3: %v\n", err)
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   fmt.Sprintf("Failed to store CSV data: %v", err),
		})
		return
	}
	fmt.Printf("DEBUG: Stored CSV data in Supabase S3 with blob name: %s for account: %s\n", blobName, accountAddress)

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "CSV data received and processed",
		Data: map[string]interface{}{
			"account_address": accountAddress,
			"data_hash":       dataHash,
			"row_count":       len(csvData) - 1, // Exclude header
			"column_count": func() int {
				if len(csvData) > 0 {
					return len(csvData[0])
				}
				return 0
			}(),
			"schema": schema,
		},
	})
}

// SubmitEncryptedCSV handles encrypted CSV data submission
func (h *Handler) SubmitEncryptedCSV(c *gin.Context) {
	var req struct {
		AccountAddress      string      `json:"account_address" binding:"required"`
		DataHash            string      `json:"data_hash" binding:"required"`
		Schema              interface{} `json:"schema" binding:"required"`
		EncryptedData       string      `json:"encrypted_data" binding:"required"`      // Base64-encoded
		EncryptionMetadata  string      `json:"encryption_metadata" binding:"required"` // Base64-encoded nonce
		PrivateKey          string      `json:"private_key"`                            // Optional: if provided, submit to blockchain
		EncryptionAlgorithm string      `json:"encryption_algorithm"`                   // Optional: defaults to "AES-256-GCM"
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "Invalid request: " + err.Error(),
		})
		return
	}

	// Decode base64 encrypted data
	encryptedBytes, err := base64.StdEncoding.DecodeString(req.EncryptedData)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   "Failed to decode encrypted data: " + err.Error(),
		})
		return
	}

	fmt.Printf("DEBUG: Encrypted CSV submitted for user %s (encrypted size: %d bytes)\n", req.AccountAddress, len(encryptedBytes))

	// Store encrypted CSV data in Supabase S3
	blobName, err := h.storageService.StoreEncryptedCSV(req.AccountAddress, encryptedBytes, req.EncryptionMetadata)
	if err != nil {
		fmt.Printf("ERROR: Failed to store encrypted CSV in Supabase S3: %v\n", err)
		c.JSON(http.StatusInternalServerError, models.Response{
			Success: false,
			Error:   fmt.Sprintf("Failed to store encrypted CSV data: %v", err),
		})
		return
	}
	fmt.Printf("DEBUG: Stored encrypted CSV data in Supabase S3 with blob name: %s for account: %s\n", blobName, req.AccountAddress)

	// Build metadata JSON from schema
	metadataJSON, err := json.Marshal(req.Schema)
	if err != nil {
		fmt.Printf("WARNING: Failed to marshal schema to JSON: %v\n", err)
		metadataJSON = []byte("{}")
	}

	// Set default encryption algorithm if not provided
	encryptionAlgorithm := req.EncryptionAlgorithm
	if encryptionAlgorithm == "" {
		encryptionAlgorithm = "AES-256-GCM"
	}

	// Submit to blockchain if private key is provided
	// Note: Frontend typically handles blockchain submission, so private_key is usually not provided
	var txHash string
	if req.PrivateKey != "" {
		fmt.Printf("DEBUG: Private key provided, submitting encrypted data to blockchain for account: %s\n", req.AccountAddress)
		txHash, err = h.aptosService.SubmitDataWithEncryption(
			req.PrivateKey,
			req.DataHash,
			string(metadataJSON),
			req.EncryptionMetadata,
			encryptionAlgorithm,
		)
		if err != nil {
			fmt.Printf("ERROR: Failed to submit to blockchain: %v\n", err)
			// Don't fail the whole request - data is already stored
			// Just log the error and continue
		} else {
			fmt.Printf("DEBUG: Successfully submitted to blockchain with transaction hash: %s\n", txHash)
		}
	} else {
		fmt.Printf("DEBUG: No private key provided - this is expected. Frontend handles blockchain submission.\n")
	}

	responseData := map[string]interface{}{
		"account_address": req.AccountAddress,
		"data_hash":       req.DataHash,
		"blob_name":       blobName,
		"encrypted":       true,
	}
	if txHash != "" {
		responseData["transaction_hash"] = txHash
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "Encrypted CSV data received and stored",
		Data:    responseData,
	})
}

// TestIndexerConnection tests the GraphQL indexer connection and returns raw query results
func (h *Handler) TestIndexerConnection(c *gin.Context) {
	// This is a debug endpoint to test if the indexer is working
	datasets, err := h.aptosService.GetMarketplaceDatasets()
	if err != nil {
		c.JSON(http.StatusOK, models.Response{
			Success: false,
			Error:   fmt.Sprintf("Indexer query failed: %v", err),
			Data: map[string]interface{}{
				"error": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: fmt.Sprintf("Indexer connection successful. Found %d datasets", len(datasets)),
		Data: map[string]interface{}{
			"dataset_count": len(datasets),
			"datasets":      datasets,
		},
	})
}

// Health check endpoint
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "Service is healthy",
	})
}
