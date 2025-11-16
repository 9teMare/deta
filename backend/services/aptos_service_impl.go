package services

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aptos-labs/aptos-go-sdk"
	"github.com/aptos-labs/aptos-go-sdk/bcs"
	"github.com/aptos-labs/aptos-go-sdk/crypto"
	"github.com/datax/backend/config"
)

// Ensure AptosServiceImpl implements AptosService interface
var _ AptosService = (*AptosServiceImpl)(nil)

// Update the AptosService to use the actual SDK
type AptosServiceImpl struct {
	client     *aptos.Client
	chainID    uint8
	httpClient *http.Client // HTTP client with timeout for API requests
}

// createHTTPClient creates an HTTP client with timeout and retry support
func createHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second, // 30 second timeout
	}
}

func NewAptosService() (*AptosServiceImpl, error) {
	// Create network config for testnet
	networkConfig := aptos.NetworkConfig{
		NodeUrl: config.AppConfig.AptosNodeURL,
		ChainId: config.AppConfig.ChainID,
	}

	client, err := aptos.NewClient(networkConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Aptos client: %w", err)
	}

	return &AptosServiceImpl{
		client:     client,
		chainID:    config.AppConfig.ChainID,
		httpClient: createHTTPClient(),
	}, nil
}

// Get account from private key hex string
func getAccountFromPrivateKey(privateKeyHex string) (*aptos.Account, error) {
	// Remove 0x prefix if present
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")

	// Parse private key
	privateKeyBytes, err := crypto.ParsePrivateKey(privateKeyHex, crypto.PrivateKeyVariantEd25519)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create Ed25519 private key
	ed25519PrivateKey := &crypto.Ed25519PrivateKey{}
	if err := ed25519PrivateKey.FromBytes(privateKeyBytes); err != nil {
		return nil, fmt.Errorf("failed to create Ed25519 private key: %w", err)
	}

	// Create account from signer
	account, err := aptos.NewAccountFromSigner(ed25519PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create account from signer: %w", err)
	}

	return account, nil
}

// Parse address from hex string
func parseAddress(addressHex string) (*aptos.AccountAddress, error) {
	addressHex = strings.TrimPrefix(addressHex, "0x")
	addressBytes, err := hex.DecodeString(addressHex)
	if err != nil {
		return nil, fmt.Errorf("invalid address hex: %w", err)
	}

	if len(addressBytes) != 32 {
		return nil, fmt.Errorf("address must be 32 bytes")
	}

	var address aptos.AccountAddress
	copy(address[:], addressBytes)
	return &address, nil
}

// Serialize argument to BCS bytes
func serializeArg(arg interface{}) ([]byte, error) {
	ser := &bcs.Serializer{}

	switch v := arg.(type) {
	case []byte:
		ser.WriteBytes(v)
	case string:
		ser.WriteString(v)
	case uint64:
		ser.U64(v)
	case *aptos.AccountAddress:
		ser.Struct(v)
	case aptos.AccountAddress:
		ser.Struct(&v)
	default:
		// Try to serialize as BCS Marshaler
		if marshaler, ok := arg.(bcs.Marshaler); ok {
			ser.Struct(marshaler)
		} else {
			return nil, fmt.Errorf("unsupported argument type: %T", arg)
		}
	}

	if err := ser.Error(); err != nil {
		return nil, err
	}

	return ser.ToBytes(), nil
}

// Submit a transaction and wait for confirmation
func (s *AptosServiceImpl) submitTransaction(
	account *aptos.Account,
	moduleAddress *aptos.AccountAddress,
	moduleName string,
	functionName string,
	args []interface{},
) (string, error) {
	// Serialize all arguments to BCS bytes
	serializedArgs := make([][]byte, 0, len(args))
	for _, arg := range args {
		argBytes, err := serializeArg(arg)
		if err != nil {
			return "", fmt.Errorf("failed to serialize argument: %w", err)
		}
		serializedArgs = append(serializedArgs, argBytes)
	}

	// Create entry function
	entryFunction := &aptos.EntryFunction{
		Module: aptos.ModuleId{
			Address: *moduleAddress,
			Name:    moduleName,
		},
		Function: functionName,
		ArgTypes: []aptos.TypeTag{},
		Args:     serializedArgs,
	}

	// Create transaction payload
	payload := aptos.TransactionPayload{
		Payload: entryFunction,
	}

	// Build, sign and submit transaction
	response, err := s.client.BuildSignAndSubmitTransaction(account, payload)
	if err != nil {
		return "", fmt.Errorf("failed to build, sign and submit transaction: %w", err)
	}

	// Wait for transaction
	_, err = s.client.WaitForTransaction(response.Hash)
	if err != nil {
		return "", fmt.Errorf("transaction failed: %w", err)
	}

	return response.Hash, nil
}

// Initialize user's data store and vault
func (s *AptosServiceImpl) InitializeUser(privateKeyHex string) (string, error) {
	account, err := getAccountFromPrivateKey(privateKeyHex)
	if err != nil {
		return "", err
	}

	moduleAddr, err := parseAddress(config.AppConfig.DataXModuleAddr)
	if err != nil {
		return "", err
	}

	return s.submitTransaction(
		account,
		moduleAddr,
		"data_registry",
		"init",
		[]interface{}{},
	)
}

// Submit data
func (s *AptosServiceImpl) SubmitData(privateKeyHex string, dataHash string, metadata string) (string, error) {
	account, err := getAccountFromPrivateKey(privateKeyHex)
	if err != nil {
		return "", err
	}

	moduleAddr, err := parseAddress(config.AppConfig.DataXModuleAddr)
	if err != nil {
		return "", err
	}

	dataHashBytes := []byte(dataHash)
	metadataBytes := []byte(metadata)

	return s.submitTransaction(
		account,
		moduleAddr,
		"data_registry",
		"submit_data",
		[]interface{}{dataHashBytes, metadataBytes},
	)
}

// Delete dataset
func (s *AptosServiceImpl) DeleteDataset(privateKeyHex string, datasetID uint64) (string, error) {
	account, err := getAccountFromPrivateKey(privateKeyHex)
	if err != nil {
		return "", err
	}

	moduleAddr, err := parseAddress(config.AppConfig.DataXModuleAddr)
	if err != nil {
		return "", err
	}

	return s.submitTransaction(
		account,
		moduleAddr,
		"data_registry",
		"delete_dataset",
		[]interface{}{datasetID},
	)
}

// Grant access
func (s *AptosServiceImpl) GrantAccess(privateKeyHex string, datasetID uint64, requester string, expiresAt uint64) (string, error) {
	account, err := getAccountFromPrivateKey(privateKeyHex)
	if err != nil {
		return "", err
	}

	moduleAddr, err := parseAddress(config.AppConfig.NetworkModuleAddr)
	if err != nil {
		return "", err
	}

	requesterAddr, err := parseAddress(requester)
	if err != nil {
		return "", err
	}

	return s.submitTransaction(
		account,
		moduleAddr,
		"AccessControl",
		"grant_access",
		[]interface{}{datasetID, requesterAddr, expiresAt},
	)
}

// Revoke access
func (s *AptosServiceImpl) RevokeAccess(privateKeyHex string, datasetID uint64, requester string) (string, error) {
	account, err := getAccountFromPrivateKey(privateKeyHex)
	if err != nil {
		return "", err
	}

	moduleAddr, err := parseAddress(config.AppConfig.NetworkModuleAddr)
	if err != nil {
		return "", err
	}

	requesterAddr, err := parseAddress(requester)
	if err != nil {
		return "", err
	}

	return s.submitTransaction(
		account,
		moduleAddr,
		"AccessControl",
		"revoke_access",
		[]interface{}{datasetID, requesterAddr},
	)
}

// Register for token
func (s *AptosServiceImpl) RegisterToken(privateKeyHex string) (string, error) {
	account, err := getAccountFromPrivateKey(privateKeyHex)
	if err != nil {
		return "", err
	}

	moduleAddr, err := parseAddress(config.AppConfig.DataXModuleAddr)
	if err != nil {
		return "", err
	}

	return s.submitTransaction(
		account,
		moduleAddr,
		"data_token",
		"register",
		[]interface{}{},
	)
}

// Mint token
func (s *AptosServiceImpl) MintToken(privateKeyHex string, recipient string, amount uint64) (string, error) {
	account, err := getAccountFromPrivateKey(privateKeyHex)
	if err != nil {
		return "", err
	}

	moduleAddr, err := parseAddress(config.AppConfig.DataXModuleAddr)
	if err != nil {
		return "", err
	}

	recipientAddr, err := parseAddress(recipient)
	if err != nil {
		return "", err
	}

	return s.submitTransaction(
		account,
		moduleAddr,
		"data_token",
		"mint",
		[]interface{}{recipientAddr, amount},
	)
}

// Read functions (view functions)
func (s *AptosServiceImpl) GetDataset(userAddress string, datasetID uint64) (interface{}, error) {
	userAddr, err := parseAddress(userAddress)
	if err != nil {
		return nil, err
	}

	moduleAddr, err := parseAddress(config.AppConfig.DataXModuleAddr)
	if err != nil {
		return nil, err
	}

	// Query the DataStore resource directly since get_dataset is not a view function
	resourceType := fmt.Sprintf("%s::data_registry::DataStore", moduleAddr.String())
	resourceURL := fmt.Sprintf("%s/v1/accounts/%s/resource/%s",
		config.AppConfig.AptosNodeURL,
		userAddr.String(),
		url.PathEscape(resourceType))

	resp, err := http.Get(resourceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to query DataStore resource: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("DataStore resource not found for user")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse the resource data
	var resourceData struct {
		Data struct {
			Datasets []struct {
				ID        interface{} `json:"id"`
				Owner     interface{} `json:"owner"`
				DataHash  interface{} `json:"data_hash"`
				Metadata  interface{} `json:"metadata"`
				CreatedAt interface{} `json:"created_at"`
				IsActive  interface{} `json:"is_active"`
			} `json:"datasets"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&resourceData); err != nil {
		return nil, fmt.Errorf("failed to decode resource data: %w", err)
	}

	// Debug: log the raw resource data structure
	fmt.Printf("DEBUG: Found %d datasets in DataStore\n", len(resourceData.Data.Datasets))

	// Find the dataset with matching ID
	for _, dataset := range resourceData.Data.Datasets {
		var id uint64
		switch v := dataset.ID.(type) {
		case float64:
			id = uint64(v)
		case string:
			parsed, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				continue
			}
			id = parsed
		case uint64:
			id = v
		default:
			continue
		}

		if id == datasetID {
			// Convert data_hash from byte arrays to hex string
			// Aptos can return byte vectors as arrays of numbers or as hex strings
			dataHashHex := "0x"
			switch v := dataset.DataHash.(type) {
			case []interface{}:
				// Array of numbers (most common format)
				for _, b := range v {
					if byteVal, ok := b.(float64); ok {
						dataHashHex += fmt.Sprintf("%02x", uint8(byteVal))
					} else if byteVal, ok := b.(uint8); ok {
						dataHashHex += fmt.Sprintf("%02x", byteVal)
					}
				}
			case string:
				// Already a hex string
				if strings.HasPrefix(v, "0x") {
					dataHashHex = v
				} else {
					dataHashHex = "0x" + v
				}
			default:
				// Try to handle other formats
				fmt.Printf("Warning: unexpected data_hash type: %T, value: %v\n", v, v)
			}

			// Convert metadata from byte arrays to string
			metadataStr := ""
			switch v := dataset.Metadata.(type) {
			case []interface{}:
				// Array of numbers - convert to UTF-8 string
				bytes := make([]byte, 0, len(v))
				for _, b := range v {
					if byteVal, ok := b.(float64); ok {
						bytes = append(bytes, uint8(byteVal))
					} else if byteVal, ok := b.(uint8); ok {
						bytes = append(bytes, byteVal)
					}
				}
				metadataStr = string(bytes)
			case string:
				// Already a string
				metadataStr = v
			default:
				fmt.Printf("Warning: unexpected metadata type: %T, value: %v\n", v, v)
			}

			var createdAt uint64
			switch v := dataset.CreatedAt.(type) {
			case float64:
				createdAt = uint64(v)
			case string:
				parsed, _ := strconv.ParseUint(v, 10, 64)
				createdAt = parsed
			case uint64:
				createdAt = v
			}

			// Parse is_active - handle both bool and string "true"/"false"
			// Default to true since datasets are created as active in the Move contract
			isActive := true
			switch v := dataset.IsActive.(type) {
			case bool:
				isActive = v
			case string:
				isActive = (v == "true" || v == "1")
			case float64:
				// Sometimes booleans come as 0/1
				isActive = (v != 0)
			case nil:
				// If nil, default to true (shouldn't happen, but be safe)
				isActive = true
			default:
				// Log unexpected type but default to true
				fmt.Printf("Warning: unexpected is_active type: %T, value: %v, defaulting to true\n", v, v)
				isActive = true
			}

			datasetInfo := map[string]interface{}{
				"data_hash":  dataHashHex,
				"metadata":   metadataStr,
				"created_at": createdAt,
				"is_active":  isActive,
			}

			return datasetInfo, nil
		}
	}

	return nil, fmt.Errorf("dataset %d not found", datasetID)
}

func (s *AptosServiceImpl) CheckAccess(owner string, datasetID uint64, requester string) (bool, error) {
	ownerAddr, err := parseAddress(owner)
	if err != nil {
		return false, err
	}

	requesterAddr, err := parseAddress(requester)
	if err != nil {
		return false, err
	}

	moduleAddr, err := parseAddress(config.AppConfig.NetworkModuleAddr)
	if err != nil {
		return false, err
	}

	// Encode arguments to BCS - need to serialize each argument separately
	ownerBytes, err := serializeArg(ownerAddr)
	if err != nil {
		return false, fmt.Errorf("failed to serialize owner address: %w", err)
	}
	datasetIDBytes, err := serializeArg(datasetID)
	if err != nil {
		return false, fmt.Errorf("failed to serialize dataset ID: %w", err)
	}
	requesterBytes, err := serializeArg(requesterAddr)
	if err != nil {
		return false, fmt.Errorf("failed to serialize requester address: %w", err)
	}

	viewPayload := &aptos.ViewPayload{
		Module: aptos.ModuleId{
			Address: *moduleAddr,
			Name:    "AccessControl",
		},
		Function: "has_access",
		ArgTypes: []aptos.TypeTag{},
		Args:     [][]byte{ownerBytes, datasetIDBytes, requesterBytes},
	}

	result, err := s.client.View(viewPayload)
	if err != nil {
		return false, fmt.Errorf("failed to call view function: %w", err)
	}

	if len(result) > 0 {
		if hasAccess, ok := result[0].(bool); ok {
			return hasAccess, nil
		}
	}

	return false, nil
}

// userRegistry is a simple in-memory registry of users who have submitted data
// In production, this would be stored in a database or use an indexer
var userRegistry = make(map[string]bool)
var userRegistryMutex sync.Mutex

// RegisterUser adds a user to the registry
// This is exported so handlers can call it
func RegisterUser(userAddress string) {
	userRegistryMutex.Lock()
	defer userRegistryMutex.Unlock()
	userRegistry[userAddress] = true
	fmt.Printf("DEBUG: Registered user %s in marketplace registry (total: %d)\n", userAddress, len(userRegistry))
}

// DiscoverUserFromVault checks if a user has datasets and registers them if they do
// This helps populate the registry even if the backend restarted
func DiscoverUserFromVault(userAddress string) bool {
	moduleAddr, err := parseAddress(config.AppConfig.DataXModuleAddr)
	if err != nil {
		return false
	}

	// Check if user has a DataStore resource
	resourceType := fmt.Sprintf("%s::data_registry::DataStore", moduleAddr.String())
	resourceURL := fmt.Sprintf("%s/v1/accounts/%s/resource/%s",
		config.AppConfig.AptosNodeURL,
		userAddress,
		url.PathEscape(resourceType))

	resp, err := http.Get(resourceURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// User has a DataStore, register them
		RegisterUser(userAddress)
		fmt.Printf("DEBUG: Discovered and registered user %s from DataStore\n", userAddress)
		return true
	}

	return false
}

// DiscoverUsersFromChain discovers users who have DataStore resources on-chain
// Uses Aptos Indexer GraphQL API to query events by type across all accounts
func (s *AptosServiceImpl) DiscoverUsersFromChain() ([]string, error) {
	moduleAddr, err := parseAddress(config.AppConfig.DataXModuleAddr)
	if err != nil {
		return nil, err
	}

	eventType := fmt.Sprintf("%s::data_registry::DataSubmitted", moduleAddr.String())

	// Try using the GraphQL Indexer API first
	if config.AppConfig.AptosIndexerURL != "" {
		users, err := s.queryUsersFromGraphQLIndexer(eventType)
		if err == nil && len(users) > 0 {
			fmt.Printf("DEBUG: Discovered %d users from GraphQL indexer\n", len(users))
			return users, nil
		}
		fmt.Printf("DEBUG: GraphQL indexer query failed, falling back to registry: %v\n", err)
	}

	// Fallback: Get users from registry and query their events
	userRegistryMutex.Lock()
	registryUsers := make([]string, 0, len(userRegistry))
	for user := range userRegistry {
		registryUsers = append(registryUsers, user)
	}
	userRegistryMutex.Unlock()

	fmt.Printf("DEBUG: Starting discovery with %d users from registry\n", len(registryUsers))

	// Query events from each registry user to discover additional users
	discoveredUsers := make(map[string]bool)

	// Add registry users to discovered set
	for _, user := range registryUsers {
		discoveredUsers[user] = true
	}

	// Query events from registry users to find any additional users mentioned in events
	// Use concurrent queries with limit
	const maxConcurrent = 5
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, userAddr := range registryUsers {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			eventsURL := fmt.Sprintf("%s/v1/accounts/%s/events/%s?limit=100",
				config.AppConfig.AptosNodeURL,
				addr,
				url.PathEscape(eventType))

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			req, err := http.NewRequestWithContext(ctx, "GET", eventsURL, nil)
			if err != nil {
				cancel()
				return
			}

			resp, err := s.httpClient.Do(req)
			cancel()

			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var eventsData struct {
					Data []struct {
						Data struct {
							User string `json:"user"`
						} `json:"data"`
					} `json:"data"`
				}

				if err := json.NewDecoder(resp.Body).Decode(&eventsData); err == nil {
					mu.Lock()
					for _, event := range eventsData.Data {
						if event.Data.User != "" {
							discoveredUsers[event.Data.User] = true
						}
					}
					mu.Unlock()
				}
			}
		}(userAddr)
	}

	wg.Wait()

	users := make([]string, 0, len(discoveredUsers))
	for user := range discoveredUsers {
		users = append(users, user)
	}

	fmt.Printf("DEBUG: Discovered %d total users (from registry + events)\n", len(users))
	return users, nil
}

// queryUsersFromGraphQLIndexer queries the Aptos Indexer GraphQL API to find all users who emitted DataSubmitted events
// Uses GraphQL to query account_transactions filtered by event type
// Reference: https://aptos.dev/build/indexer/indexer-api/indexer-reference
func (s *AptosServiceImpl) queryUsersFromGraphQLIndexer(eventType string) ([]string, error) {
	// GraphQL query to find all transactions that emitted DataSubmitted events
	// Query account_transactions table and filter by event type
	// The account_address field gives us the user who submitted the data
	graphQLQuery := fmt.Sprintf(`query {
		account_transactions(
			where: {
				events: {
					type: { _eq: "%s" }
				}
			},
			limit: 1000,
			order_by: { transaction_version: desc }
		) {
			account_address
			events(
				where: {
					type: { _eq: "%s" }
				}
			) {
				data
			}
		}
	}`, eventType, eventType)

	// Prepare GraphQL request
	requestBody := map[string]interface{}{
		"query": graphQLQuery,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	// Retry logic: try up to 3 times with exponential backoff
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			fmt.Printf("DEBUG: Retrying GraphQL indexer query (attempt %d/%d) after %v\n", attempt+1, 3, backoff)
			time.Sleep(backoff)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		req, err := http.NewRequestWithContext(ctx, "POST", config.AppConfig.AptosIndexerURL, strings.NewReader(string(jsonBody)))
		if err != nil {
			cancel()
			lastErr = err
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := s.httpClient.Do(req)
		cancel()

		if err != nil {
			lastErr = fmt.Errorf("GraphQL request failed: %w", err)
			fmt.Printf("DEBUG: GraphQL request error (attempt %d): %v\n", attempt+1, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("GraphQL returned status %d: %s", resp.StatusCode, string(bodyBytes))
			fmt.Printf("DEBUG: GraphQL returned status %d (attempt %d): %s\n", resp.StatusCode, attempt+1, string(bodyBytes))
			continue
		}

		var graphQLResponse struct {
			Data struct {
				AccountTransactions []struct {
					AccountAddress string `json:"account_address"`
					Events         []struct {
						Data json.RawMessage `json:"data"`
					} `json:"events"`
				} `json:"account_transactions"`
			} `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&graphQLResponse); err != nil {
			lastErr = fmt.Errorf("failed to decode GraphQL response: %w", err)
			fmt.Printf("DEBUG: Failed to decode GraphQL response (attempt %d): %v\n", attempt+1, err)
			continue
		}

		// Check for GraphQL errors
		if len(graphQLResponse.Errors) > 0 {
			errorMessages := make([]string, len(graphQLResponse.Errors))
			for i, err := range graphQLResponse.Errors {
				errorMessages[i] = err.Message
			}
			lastErr = fmt.Errorf("GraphQL errors: %s", strings.Join(errorMessages, "; "))
			fmt.Printf("DEBUG: GraphQL errors (attempt %d): %v\n", attempt+1, errorMessages)
			continue
		}

		// Extract users from events
		userSet := make(map[string]bool)
		for _, tx := range graphQLResponse.Data.AccountTransactions {
			// Add the account address that emitted the event
			if tx.AccountAddress != "" {
				userSet[tx.AccountAddress] = true
			}

			// Also extract user from event data
			for _, event := range tx.Events {
				var eventData struct {
					User string `json:"user"`
				}
				if err := json.Unmarshal(event.Data, &eventData); err == nil {
					if eventData.User != "" {
						userSet[eventData.User] = true
					}
				}
			}
		}

		users := make([]string, 0, len(userSet))
		for user := range userSet {
			users = append(users, user)
		}

		fmt.Printf("DEBUG: Successfully queried GraphQL indexer, found %d users\n", len(users))
		return users, nil
	}

	return nil, fmt.Errorf("GraphQL indexer query failed after 3 attempts: %w", lastErr)
}

// GetMarketplaceDatasets queries DataStore resources directly from the blockchain
// It discovers users from chain events and queries their DataStore resources to get all datasets
// This approach fetches data directly from on-chain state, not from memory
func (s *AptosServiceImpl) GetMarketplaceDatasets() ([]interface{}, error) {
	moduleAddr, err := parseAddress(config.AppConfig.DataXModuleAddr)
	if err != nil {
		return nil, err
	}

	// Step 1: Discover users from chain (query events from module address)
	fmt.Printf("DEBUG: Discovering users from blockchain...\n")
	users, err := s.DiscoverUsersFromChain()
	if err != nil {
		fmt.Printf("DEBUG: Error discovering users: %v\n", err)
		users = []string{}
	}

	// Step 2: Also include users from registry (for backward compatibility)
	userRegistryMutex.Lock()
	for user := range userRegistry {
		// Add to users list if not already present
		found := false
		for _, u := range users {
			if u == user {
				found = true
				break
			}
		}
		if !found {
			users = append(users, user)
		}
	}
	userRegistryMutex.Unlock()

	fmt.Printf("DEBUG: Total users to query: %d (from chain: %d, from registry: %d)\n",
		len(users), len(users), len(userRegistry))

	if len(users) == 0 {
		fmt.Printf("DEBUG: No users found. Datasets may not exist yet, or events are not stored at module address.\n")
		return []interface{}{}, nil
	}

	// Step 3: Query DataStore resources directly from each discovered user account
	// This is more reliable than querying events, as it gets data directly from on-chain state
	// Use concurrent requests with proper error handling
	datasets := make([]interface{}, 0)
	seenDatasets := make(map[string]bool) // Track owner+datasetID to avoid duplicates
	datasetsMutex := sync.Mutex{}         // Protect datasets slice

	resourceType := fmt.Sprintf("%s::data_registry::DataStore", moduleAddr.String())

	// Use a worker pool to query users concurrently (max 5 concurrent requests)
	const maxConcurrent = 5
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, userAddr := range users {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			fmt.Printf("DEBUG: Querying DataStore resource from user: %s\n", addr)

			// Query DataStore resource directly from chain with retry
			resourceURL := fmt.Sprintf("%s/v1/accounts/%s/resource/%s",
				config.AppConfig.AptosNodeURL,
				addr,
				url.PathEscape(resourceType))

			var resp *http.Response
			var err error

			// Retry up to 2 times
			for attempt := 0; attempt < 2; attempt++ {
				if attempt > 0 {
					time.Sleep(500 * time.Millisecond)
				}

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				req, reqErr := http.NewRequestWithContext(ctx, "GET", resourceURL, nil)
				if reqErr != nil {
					cancel()
					err = reqErr
					continue
				}

				resp, err = s.httpClient.Do(req)
				cancel()

				if err == nil && resp.StatusCode == http.StatusOK {
					break
				}

				if resp != nil {
					resp.Body.Close()
				}
			}

			if err != nil {
				fmt.Printf("DEBUG: Failed to query DataStore from %s after retries: %v\n", addr, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				fmt.Printf("DEBUG: No DataStore found for user %s\n", addr)
				return
			}

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("DEBUG: DataStore query returned status %d for user %s\n", resp.StatusCode, addr)
				return
			}

			// Parse the DataStore resource
			var resourceData struct {
				Data struct {
					Datasets []struct {
						ID        interface{} `json:"id"`
						Owner     interface{} `json:"owner"`
						DataHash  interface{} `json:"data_hash"`
						Metadata  interface{} `json:"metadata"`
						CreatedAt interface{} `json:"created_at"`
						IsActive  interface{} `json:"is_active"`
					} `json:"datasets"`
				} `json:"data"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&resourceData); err != nil {
				fmt.Printf("DEBUG: Failed to decode DataStore from %s: %v\n", addr, err)
				return
			}

			fmt.Printf("DEBUG: Found %d datasets in DataStore for user %s\n", len(resourceData.Data.Datasets), addr)

			// Process each dataset from the DataStore
			userDatasets := make([]interface{}, 0)

			for _, dataset := range resourceData.Data.Datasets {
				// Parse dataset ID
				var datasetID uint64
				switch v := dataset.ID.(type) {
				case float64:
					datasetID = uint64(v)
				case string:
					parsed, err := strconv.ParseUint(v, 10, 64)
					if err != nil {
						continue
					}
					datasetID = parsed
				case uint64:
					datasetID = v
				default:
					continue
				}

				// Create unique key
				key := fmt.Sprintf("%s-%d", addr, datasetID)

				// Check if already seen (thread-safe check)
				datasetsMutex.Lock()
				if seenDatasets[key] {
					datasetsMutex.Unlock()
					continue
				}
				seenDatasets[key] = true
				datasetsMutex.Unlock()

				// Parse data_hash
				var dataHash string
				switch v := dataset.DataHash.(type) {
				case string:
					dataHash = v
				case []interface{}:
					// Byte array - convert to hex
					bytes := make([]byte, 0, len(v))
					for _, b := range v {
						if num, ok := b.(float64); ok {
							bytes = append(bytes, byte(num))
						}
					}
					dataHash = "0x" + hex.EncodeToString(bytes)
				default:
					dataHash = fmt.Sprintf("%v", v)
				}

				// Parse metadata
				var metadata string
				switch v := dataset.Metadata.(type) {
				case string:
					metadata = v
				case []interface{}:
					// Byte array - try to decode as UTF-8
					bytes := make([]byte, 0, len(v))
					for _, b := range v {
						if num, ok := b.(float64); ok {
							bytes = append(bytes, byte(num))
						}
					}
					metadata = string(bytes)
				default:
					metadata = fmt.Sprintf("%v", v)
				}

				// Parse created_at
				var createdAt uint64
				switch v := dataset.CreatedAt.(type) {
				case float64:
					createdAt = uint64(v)
				case string:
					parsed, _ := strconv.ParseUint(v, 10, 64)
					createdAt = parsed
				case uint64:
					createdAt = v
				}

				// Parse is_active
				isActive := true
				switch v := dataset.IsActive.(type) {
				case bool:
					isActive = v
				case string:
					isActive = (v == "true" || v == "1")
				case float64:
					isActive = (v != 0)
				}

				// Only include active datasets
				if !isActive {
					continue
				}

				// Create dataset info map
				datasetInfo := map[string]interface{}{
					"id":         datasetID,
					"owner":      addr,
					"data_hash":  dataHash,
					"metadata":   metadata,
					"created_at": createdAt,
					"is_active":  isActive,
				}

				userDatasets = append(userDatasets, datasetInfo)
			}

			// Thread-safe append to main datasets slice
			datasetsMutex.Lock()
			datasets = append(datasets, userDatasets...)
			datasetsMutex.Unlock()
		}(userAddr)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	fmt.Printf("DEBUG: Marketplace returning %d datasets from blockchain (DataStore resources)\n", len(datasets))
	return datasets, nil
}

// accessRequestStore stores access requests
// In production, this would be in a database or on-chain
var accessRequestStore = make(map[string][]map[string]interface{})
var accessRequestStoreMutex sync.Mutex

// RequestAccess stores an access request
func RequestAccess(ownerAddress string, datasetID uint64, requesterAddress string, message string) {
	accessRequestStoreMutex.Lock()
	defer accessRequestStoreMutex.Unlock()

	key := fmt.Sprintf("%s-%d", ownerAddress, datasetID)
	request := map[string]interface{}{
		"dataset_id":   datasetID,
		"requester":    requesterAddress,
		"message":      message,
		"requested_at": fmt.Sprintf("%d", uint64(0)), // Would use timestamp in production
	}

	accessRequestStore[key] = append(accessRequestStore[key], request)
}

// GetAccessRequests returns access requests for a dataset owner
func (s *AptosServiceImpl) GetAccessRequests(ownerAddress string) ([]interface{}, error) {
	accessRequestStoreMutex.Lock()
	defer accessRequestStoreMutex.Unlock()

	requests := make([]interface{}, 0)
	for key, reqs := range accessRequestStore {
		// Check if this key starts with the owner address
		if len(key) > len(ownerAddress) && key[:len(ownerAddress)] == ownerAddress {
			for _, req := range reqs {
				requests = append(requests, req)
			}
		}
	}

	return requests, nil
}

func (s *AptosServiceImpl) GetUserVault(userAddress string) ([]uint64, error) {
	userAddr, err := parseAddress(userAddress)
	if err != nil {
		return nil, err
	}

	moduleAddr, err := parseAddress(config.AppConfig.NetworkModuleAddr)
	if err != nil {
		return nil, err
	}

	// Construct the resource type: {moduleAddress}::UserVault::Vault
	resourceType := fmt.Sprintf("%s::UserVault::Vault", moduleAddr.String())

	// Query the resource directly via REST API
	resourceURL := fmt.Sprintf("%s/v1/accounts/%s/resource/%s",
		config.AppConfig.AptosNodeURL,
		userAddr.String(),
		url.PathEscape(resourceType))

	resp, err := http.Get(resourceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to query resource: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Resource doesn't exist, return empty array
		return []uint64{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse the response
	var resourceData struct {
		Data struct {
			Datasets interface{} `json:"datasets"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&resourceData); err != nil {
		return nil, fmt.Errorf("failed to decode resource data: %w", err)
	}

	// Convert the datasets array - it might be []interface{} or []string
	datasetIDs := make([]uint64, 0)

	// The datasets field might be an array of numbers or strings
	if datasetsInterface, ok := resourceData.Data.Datasets.([]interface{}); ok {
		for _, item := range datasetsInterface {
			var id uint64
			switch v := item.(type) {
			case float64:
				id = uint64(v)
			case string:
				parsed, err := strconv.ParseUint(v, 10, 64)
				if err != nil {
					continue
				}
				id = parsed
			case uint64:
				id = v
			default:
				continue
			}
			datasetIDs = append(datasetIDs, id)
		}
	}

	return datasetIDs, nil
}

// IsAccountInitialized checks if the user has initialized their DataStore
// We check by trying to query the Vault resource directly
func (s *AptosServiceImpl) IsAccountInitialized(userAddress string) (bool, error) {
	userAddr, err := parseAddress(userAddress)
	if err != nil {
		return false, err
	}

	moduleAddr, err := parseAddress(config.AppConfig.NetworkModuleAddr)
	if err != nil {
		return false, err
	}

	// Construct the resource type: {moduleAddress}::UserVault::Vault
	resourceType := fmt.Sprintf("%s::UserVault::Vault", moduleAddr.String())

	// Check if the Vault resource exists by querying it directly via REST API
	// Build the resource URL - use PathEscape for path segments
	resourceURL := fmt.Sprintf("%s/v1/accounts/%s/resource/%s",
		config.AppConfig.AptosNodeURL,
		userAddr.String(),
		url.PathEscape(resourceType))

	// Make HTTP request to check if resource exists
	// This is a simpler approach than using view functions
	resp, err := http.Get(resourceURL)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()

	// If we get 200, the resource exists (account is initialized)
	// If we get 404, the resource doesn't exist (account not initialized)
	if resp.StatusCode == http.StatusOK {
		return true, nil
	} else if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	// Other status codes indicate an error
	return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}
