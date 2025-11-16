package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
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

// csvDataStore stores CSV data by hash for retrieval
// In production, this would be in a database
var csvDataStore = make(map[string][][]string)
var csvDataStoreMutex sync.Mutex

// hashToBlobName maps data hash (SHA-256) to Shelby blob name
var hashToBlobName = make(map[string]string)
var hashToBlobNameMutex sync.Mutex

// storeCSVData stores CSV data by hash
func storeCSVData(hash string, data [][]string) {
	csvDataStoreMutex.Lock()
	defer csvDataStoreMutex.Unlock()
	csvDataStore[hash] = data
}

// getCSVData retrieves CSV data by hash
func getCSVData(hash string) ([][]string, bool) {
	csvDataStoreMutex.Lock()
	defer csvDataStoreMutex.Unlock()
	data, exists := csvDataStore[hash]
	return data, exists
}

// storeHashToBlobName stores the mapping from data hash to blob name
func storeHashToBlobName(dataHash string, blobName string) {
	hashToBlobNameMutex.Lock()
	defer hashToBlobNameMutex.Unlock()

	// Normalize data hash: remove 0x prefix for consistent storage
	normalizedHash := strings.TrimPrefix(dataHash, "0x")

	// Store with normalized hash (without 0x)
	hashToBlobName[normalizedHash] = blobName

	// Also store with 0x prefix for backward compatibility
	if !strings.HasPrefix(dataHash, "0x") {
		hashToBlobName["0x"+dataHash] = blobName
	}

	fmt.Printf("DEBUG: Stored mapping: dataHash=%s (normalized=%s) -> blobName=%s\n", dataHash, normalizedHash, blobName)
}

// getBlobNameFromHash retrieves the blob name for a given data hash
func getBlobNameFromHash(dataHash string) (string, bool) {
	hashToBlobNameMutex.Lock()
	defer hashToBlobNameMutex.Unlock()

	// Try exact match first
	blobName, exists := hashToBlobName[dataHash]
	if exists {
		return blobName, true
	}

	// Try without 0x prefix
	hashWithoutPrefix := strings.TrimPrefix(dataHash, "0x")
	if hashWithoutPrefix != dataHash {
		blobName, exists = hashToBlobName[hashWithoutPrefix]
		if exists {
			return blobName, true
		}
	}

	// Try with 0x prefix
	hashWithPrefix := "0x" + dataHash
	if !strings.HasPrefix(dataHash, "0x") {
		blobName, exists = hashToBlobName[hashWithPrefix]
		if exists {
			return blobName, true
		}
	}

	// Debug: log all available mappings
	fmt.Printf("DEBUG: Available mappings count: %d\n", len(hashToBlobName))
	if len(hashToBlobName) > 0 {
		fmt.Printf("DEBUG: Sample mappings (first 3):\n")
		count := 0
		for k, v := range hashToBlobName {
			if count < 3 {
				fmt.Printf("  %s -> %s\n", k, v)
				count++
			}
		}
	}

	return "", false
}

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

// SubmitData submits data to the registry
func (h *Handler) SubmitData(c *gin.Context) {
	var req models.SubmitDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	txHash, err := h.aptosService.SubmitData(req.PrivateKey, req.DataHash, req.Metadata)
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
			Message: "Data submitted successfully",
		},
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

	// Try to discover user from their vault first (checks if they have DataStore)
	discovered := services.DiscoverUserFromVault(req.UserAddress)
	if !discovered {
		// If not discovered, just register them anyway
		services.RegisterUser(req.UserAddress)
		fmt.Printf("DEBUG: Manually registered user %s in marketplace registry\n", req.UserAddress)
	}

	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "User registered for marketplace",
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

	// Try to get CSV data from in-memory store first
	csvData, exists := getCSVData(req.DataHash)
	if !exists {
		// If not found in memory, try to retrieve from Shelby
		// First, check if we have a blob name mapping for this data hash
		blobName, hasMapping := getBlobNameFromHash(req.DataHash)
		if hasMapping {
			fmt.Printf("DEBUG: Found blob name mapping: dataHash=%s -> blobName=%s\n", req.DataHash, blobName)
			var err error
			csvData, err = h.storageService.RetrieveCSV(req.Owner, blobName)
			if err != nil {
				fmt.Printf("ERROR: Failed to retrieve from Shelby with blob name %s: %v\n", blobName, err)
				c.JSON(http.StatusNotFound, models.Response{
					Success: false,
					Error:   fmt.Sprintf("CSV data not found on Shelby: %v", err),
				})
				return
			}
			fmt.Printf("DEBUG: Successfully retrieved CSV from Shelby using blob name: %s\n", blobName)
		} else {
			// If no mapping exists, try using the data hash directly (in case it's already a blob name)
			// This handles legacy cases where blob name was stored as the data hash
			// Also try if blob name contains "/" (Supabase format: {account}/{timestamp}_{hash}.csv)
			if strings.HasPrefix(req.DataHash, "csv_") || strings.Contains(req.DataHash, "/") {
				fmt.Printf("DEBUG: Data hash looks like a blob name, trying direct retrieval: %s\n", req.DataHash)
				var err error
				csvData, err = h.storageService.RetrieveCSV(req.Owner, req.DataHash)
				if err != nil {
					fmt.Printf("ERROR: Failed to retrieve from storage with direct hash: %v\n", err)
					c.JSON(http.StatusNotFound, models.Response{
						Success: false,
						Error:   fmt.Sprintf("CSV data not found in storage: %v", err),
					})
					return
				}
				fmt.Printf("DEBUG: Successfully retrieved CSV from storage using direct hash: %s\n", req.DataHash)
			} else {
				fmt.Printf("ERROR: No CSV data found in memory and no blob name mapping for data hash: %s\n", req.DataHash)
				fmt.Printf("DEBUG: Attempting to find blob by listing S3 objects for owner: %s\n", req.Owner)

				// Last resort: try to list objects in S3 bucket and find matching blob
				// The blob name format is: {owner}/{timestamp}_{hash}.csv
				// Since we don't have the mapping, we'll list all objects for this owner
				// and try to retrieve the most recent CSV file
				if supabaseService, ok := h.storageService.(interface {
					FindBlobByPattern(accountAddress string, pattern string) (string, error)
				}); ok {
					// Try with empty pattern to list all objects for this owner and get the most recent CSV
					blobName, err := supabaseService.FindBlobByPattern(req.Owner, "")
					if err == nil {
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
						// Store the mapping for future use
						storeHashToBlobName(req.DataHash, blobName)
						fmt.Printf("DEBUG: Successfully retrieved CSV and stored mapping: %s -> %s\n", req.DataHash, blobName)
					} else {
						fmt.Printf("ERROR: Listing objects failed: %v\n", err)
						c.JSON(http.StatusNotFound, models.Response{
							Success: false,
							Error:   fmt.Sprintf("CSV data not found. Data hash: %s. The mapping was lost (backend may have restarted). Error: %v", req.DataHash, err),
						})
						return
					}
				} else {
					c.JSON(http.StatusNotFound, models.Response{
						Success: false,
						Error:   fmt.Sprintf("CSV data not found. Data hash: %s. The file may not have been stored, or the mapping is missing.", req.DataHash),
					})
					return
				}
			}
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

	// Register user in marketplace registry (important for marketplace discovery)
	services.RegisterUser(accountAddress)
	fmt.Printf("DEBUG: Registered user %s in marketplace registry\n", accountAddress)

	// Store CSV data on Shelby
	blobName, err := h.storageService.StoreCSV(accountAddress, csvData)
	if err != nil {
		fmt.Printf("WARNING: Failed to store CSV on Shelby: %v, falling back to in-memory storage\n", err)
		// Fallback to in-memory storage
		storeCSVData(dataHash, csvData)
	} else {
		fmt.Printf("DEBUG: Stored CSV data on Shelby with blob name: %s for account: %s\n", blobName, accountAddress)
		// Store mapping from data hash (SHA-256) to blob name
		storeHashToBlobName(dataHash, blobName)
		// Also store in-memory with both keys for backward compatibility
		storeCSVData(blobName, csvData)
		storeCSVData(dataHash, csvData)
	}

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

// Health check endpoint
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, models.Response{
		Success: true,
		Message: "Service is healthy",
	})
}
