package services

import (
	"bytes"
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
	"github.com/hasura/go-graphql-client"
)

// Ensure AptosServiceImpl implements AptosService interface
var _ AptosService = (*AptosServiceImpl)(nil)

// Update the AptosService to use the actual SDK
type AptosServiceImpl struct {
	client        *aptos.Client
	chainID       uint8
	httpClient    *http.Client    // HTTP client with timeout for API requests
	graphqlClient *graphql.Client // GraphQL client for indexer queries
}

// authTransport wraps http.Transport to add Authorization header
type authTransport struct {
	apiKey string
	base   http.RoundTripper
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	fmt.Printf("DEBUG: Added Authorization header to request (key length: %d)\n", len(t.apiKey))
	if t.base == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	return t.base.RoundTrip(req)
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

	// Create GraphQL client if indexer URL is configured
	var graphqlClient *graphql.Client
	if config.AppConfig.AptosIndexerURL != "" {
		apiKey := strings.TrimSpace(config.AppConfig.AptosIndexerAPIKey)

		// Create HTTP client with custom transport that adds Authorization header
		var httpClient *http.Client
		if apiKey != "" {
			fmt.Printf("DEBUG: Initializing GraphQL client with API key (length: %d chars)\n", len(apiKey))
			// Create a transport that adds the Authorization header
			transport := &authTransport{
				apiKey: apiKey,
				base:   http.DefaultTransport,
			}
			httpClient = &http.Client{
				Timeout:   30 * time.Second,
				Transport: transport,
			}
		} else {
			fmt.Printf("WARNING: APTOS_INDEXER_API_KEY is empty but indexer URL is set\n")
			httpClient = &http.Client{Timeout: 30 * time.Second}
		}

		graphqlClient = graphql.NewClient(config.AppConfig.AptosIndexerURL, httpClient)
	}

	return &AptosServiceImpl{
		client:        client,
		chainID:       config.AppConfig.ChainID,
		httpClient:    createHTTPClient(),
		graphqlClient: graphqlClient,
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

// Submit data (legacy - uses empty encryption fields)
func (s *AptosServiceImpl) SubmitData(privateKeyHex string, dataHash string, metadata string) (string, error) {
	emptyEncryption := ""
	emptyAlgorithm := ""
	return s.SubmitDataWithEncryption(privateKeyHex, dataHash, metadata, emptyEncryption, emptyAlgorithm)
}

// SubmitDataWithEncryption submits data with encryption metadata
func (s *AptosServiceImpl) SubmitDataWithEncryption(privateKeyHex string, dataHash string, metadata string, encryptionMetadata string, encryptionAlgorithm string) (string, error) {
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
	encryptionMetadataBytes := []byte(encryptionMetadata)
	encryptionAlgorithmBytes := []byte(encryptionAlgorithm)

	return s.submitTransaction(
		account,
		moduleAddr,
		"data_registry",
		"submit_data",
		[]interface{}{dataHashBytes, metadataBytes, encryptionMetadataBytes, encryptionAlgorithmBytes},
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

	nodeURL := strings.TrimSuffix(config.AppConfig.AptosNodeURL, "/")
	resourceURL := fmt.Sprintf("%s/v1/accounts/%s/resource/%s",
		nodeURL,
		userAddr.String(),
		url.PathEscape(resourceType))

	fmt.Printf("DEBUG: Querying resource at URL: %s\n", resourceURL)

	// Retry logic with exponential backoff for rate limiting
	var resp *http.Response
	var bodyBytes []byte
	var lastErr error
	var lastStatusCode int

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			fmt.Printf("DEBUG: Retrying GetDataset query (attempt %d/3) after %v\n", attempt+1, backoff)
			time.Sleep(backoff)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", resourceURL, nil)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}

		resp, err = s.httpClient.Do(req)
		cancel()

		if err != nil {
			lastErr = fmt.Errorf("failed to query DataStore resource: %w", err)
			fmt.Printf("DEBUG: GetDataset request error (attempt %d): %v\n", attempt+1, err)
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}

		// Read response body before checking status
		bodyBytes, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		lastStatusCode = resp.StatusCode

		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			fmt.Printf("DEBUG: Failed to read response (attempt %d): %v\n", attempt+1, err)
			bodyBytes = nil // Clear bodyBytes on error
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			fmt.Printf("DEBUG: DataStore resource not found for user %s\n", userAddr.String())
			return nil, fmt.Errorf("DataStore resource not found for user")
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("rate limited (429)")
			fmt.Printf("DEBUG: Rate limited (429) on attempt %d, will retry. Body: %s\n", attempt+1, string(bodyBytes))
			bodyBytes = nil // Clear bodyBytes before retry
			// Wait longer for rate limits
			if attempt < 2 {
				time.Sleep(5 * time.Second)
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			fmt.Printf("DEBUG: GetDataset returned status %d (attempt %d). Body: %s\n", resp.StatusCode, attempt+1, string(bodyBytes))
			bodyBytes = nil // Clear bodyBytes before retry
			// Don't retry on client errors (4xx) except 429
			if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
				return nil, lastErr
			}
			continue
		}

		// Success - break out of retry loop
		fmt.Printf("DEBUG: GetDataset succeeded on attempt %d\n", attempt+1)
		break
	}

	if resp == nil {
		return nil, fmt.Errorf("failed to query DataStore resource after retries: %w", lastErr)
	}

	if lastStatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to query DataStore resource: status %d, error: %w", lastStatusCode, lastErr)
	}

	if len(bodyBytes) == 0 {
		return nil, fmt.Errorf("empty response body from DataStore resource query")
	}

	// Log response body for debugging (first 500 chars)
	bodyPreview := string(bodyBytes)
	if len(bodyPreview) > 500 {
		bodyPreview = bodyPreview[:500] + "..."
	}
	fmt.Printf("DEBUG: GetDataset response body (first 500 chars): %s\n", bodyPreview)

	// Parse the resource data from the already-read body bytes
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

	if err := json.Unmarshal(bodyBytes, &resourceData); err != nil {
		fmt.Printf("DEBUG: Failed to unmarshal response body. Length: %d bytes. Error: %v\n", len(bodyBytes), err)
		fmt.Printf("DEBUG: Response body (full): %s\n", string(bodyBytes))
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

// Note: All user discovery is now done directly from the blockchain
// No in-memory registry is used - we query DataStore resources directly

// DiscoverUsersFromChain discovers users who have DataStore resources on-chain
// Uses Aptos Indexer GraphQL API to query events by type across all accounts
func (s *AptosServiceImpl) DiscoverUsersFromChain() ([]string, error) {
	moduleAddr, err := parseAddress(config.AppConfig.DataXModuleAddr)
	if err != nil {
		return nil, err
	}

	eventType := fmt.Sprintf("%s::data_registry::DataSubmitted", moduleAddr.String())

	// Try using the GraphQL Indexer API (if configured)
	// Even if USE_INDEXER is false, we'll try it as a fallback since without it we can't discover users
	if config.AppConfig.AptosIndexerURL != "" {
		if config.AppConfig.UseIndexer {
			fmt.Printf("DEBUG: Indexer is enabled, attempting to query GraphQL indexer...\n")
		} else {
			fmt.Printf("DEBUG: Indexer is disabled but will try as fallback (required for user discovery)...\n")
		}

		users, err := s.queryUsersFromGraphQLIndexer(eventType)
		if err == nil && len(users) > 0 {
			fmt.Printf("DEBUG: Discovered %d users from GraphQL indexer\n", len(users))
			return users, nil
		}
		// Log the error but continue with fallback
		if config.AppConfig.UseIndexer {
			fmt.Printf("DEBUG: GraphQL indexer query failed, trying fallback: %v\n", err)
		} else {
			fmt.Printf("DEBUG: GraphQL indexer query failed (indexer disabled): %v\n", err)
		}
	} else {
		fmt.Printf("DEBUG: GraphQL indexer URL not configured\n")
	}

	// Fallback: Try to query events from the module address
	// Note: In Aptos, events are stored on the account that emitted them, not the module
	// However, some events might be queryable from the module address
	// This is a best-effort fallback when indexer is unavailable
	discoveredUsers := make(map[string]bool)

	fmt.Printf("DEBUG: Attempting fallback: query events from module address\n")

	// Try querying events from the module address
	eventsURL := fmt.Sprintf("%s/v1/accounts/%s/events/%s?limit=1000",
		config.AppConfig.AptosNodeURL,
		moduleAddr.String(),
		url.PathEscape(eventType))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	req, err := http.NewRequestWithContext(ctx, "GET", eventsURL, nil)
	if err == nil {
		resp, err := s.httpClient.Do(req)
		cancel()

		if err == nil {
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
					for _, event := range eventsData.Data {
						if event.Data.User != "" {
							discoveredUsers[event.Data.User] = true
						}
					}
					fmt.Printf("DEBUG: Discovered %d users from module events\n", len(discoveredUsers))
				} else {
					fmt.Printf("DEBUG: Failed to decode module events: %v\n", err)
				}
			} else if resp.StatusCode == http.StatusNotFound {
				fmt.Printf("DEBUG: Module events not found (events are stored on user accounts, not module)\n")
			} else {
				fmt.Printf("DEBUG: Module events query returned status %d\n", resp.StatusCode)
			}
		} else {
			cancel()
			fmt.Printf("DEBUG: Failed to query module events: %v\n", err)
		}
	} else {
		cancel()
		fmt.Printf("DEBUG: Failed to create request for module events: %v\n", err)
	}

	// Note: Without an indexer, we cannot discover all users because:
	// 1. Events are stored on user accounts, not the module
	// 2. We cannot enumerate all accounts on Aptos
	// 3. We need either an indexer or a registry of known users
	if len(discoveredUsers) == 0 {
		fmt.Printf("WARNING: No users discovered. Without indexer, user discovery is not possible.\n")
		fmt.Printf("WARNING: Please enable indexer (USE_INDEXER=true) or the marketplace will be empty.\n")
	}

	users := make([]string, 0, len(discoveredUsers))
	for user := range discoveredUsers {
		users = append(users, user)
	}

	fmt.Printf("DEBUG: Discovered %d total users (from registry + events)\n", len(users))
	return users, nil
}

// queryUsersFromGraphQLIndexer queries the Aptos Indexer GraphQL API to find all users who emitted DataSubmitted events
// Queries events directly with event type filter
// Reference: https://aptos.dev/build/indexer/indexer-api/indexer-reference
func (s *AptosServiceImpl) queryUsersFromGraphQLIndexer(eventType string) ([]string, error) {
	// Try 'events' field first (without _v2 suffix)
	// The Aptos GraphQL indexer uses 'events' as the table name
	// graphQLQuery := fmt.Sprintf(`query GetDataSubmittedEvents {
	// 	events(
	// 		where: {
	// 			type: { _eq: "%s" }
	// 		},
	// 		limit: 1000,
	// 		order_by: { transaction_version: desc }
	// 	) {
	// 		account_address
	// 		data
	// 	}
	// }`, eventType)
	graphQLQuery := `query MyQuery {
		datasets {
			data_hash
			dataset_id
			owner
			is_active
			metadata
			created_at
			encryption_metadata
			encryption_algorithm
		}
	}
	`

	// Prepare GraphQL request
	requestBody := map[string]interface{}{
		"query": graphQLQuery,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	fmt.Printf("DEBUG: GraphQL query: %s\n", graphQLQuery)
	fmt.Printf("DEBUG: Querying indexer at: %s\n", config.AppConfig.AptosIndexerURL)

	// Retry logic: try up to 3 times with exponential backoff
	// Add initial delay to avoid rate limiting
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second * 3
			fmt.Printf("DEBUG: Retrying GraphQL indexer query (attempt %d/%d) after %v\n", attempt+1, 3, backoff)
			time.Sleep(backoff)
		} else {
			// Small initial delay to avoid hitting rate limits on first request
			time.Sleep(100 * time.Millisecond)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		req, err := http.NewRequestWithContext(ctx, "POST", config.AppConfig.AptosIndexerURL, strings.NewReader(string(jsonBody)))
		if err != nil {
			cancel()
			lastErr = err
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "DataX-Backend/1.0")

		// Add API key if configured
		apiKey := strings.TrimSpace(config.AppConfig.AptosIndexerAPIKey)
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
			fmt.Printf("DEBUG: Added Authorization header to manual HTTP request (key length: %d)\n", len(apiKey))
		} else {
			fmt.Printf("WARNING: No API key set for GraphQL request\n")
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("GraphQL request failed: %w", err)
			fmt.Printf("DEBUG: GraphQL request error (attempt %d): %v\n", attempt+1, err)
			continue
		}

		// Read response body before checking status to capture error details
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel() // Cancel after reading body

		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", readErr)
			fmt.Printf("DEBUG: Failed to read response (attempt %d): %v\n", attempt+1, readErr)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("GraphQL returned status %d: %s", resp.StatusCode, string(bodyBytes))
			fmt.Printf("DEBUG: GraphQL returned status %d (attempt %d): %s\n", resp.StatusCode, attempt+1, string(bodyBytes))

			// If rate limited (429), wait longer before retry
			if resp.StatusCode == http.StatusTooManyRequests {
				fmt.Printf("DEBUG: Rate limited, waiting 5 seconds before next retry\n")
				time.Sleep(5 * time.Second)
			}
			continue
		}

		fmt.Printf("DEBUG: GraphQL response received (attempt %d), status: %d\n", attempt+1, resp.StatusCode)

		// Parse response dynamically to handle both events and datasets queries
		var rawResponse map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &rawResponse); err != nil {
			lastErr = fmt.Errorf("failed to decode GraphQL response: %w", err)
			fmt.Printf("DEBUG: Failed to decode GraphQL response (attempt %d): %v\n", attempt+1, err)
			fmt.Printf("DEBUG: Response body: %s\n", string(bodyBytes))
			continue
		}

		// Check for GraphQL errors
		if errors, ok := rawResponse["errors"].([]interface{}); ok && len(errors) > 0 {
			errorMessages := make([]string, len(errors))
			for i, err := range errors {
				if errMap, ok := err.(map[string]interface{}); ok {
					if msg, ok := errMap["message"].(string); ok {
						errorMessages[i] = msg
					}
				}
			}
			lastErr = fmt.Errorf("GraphQL errors: %s", strings.Join(errorMessages, "; "))
			fmt.Printf("DEBUG: GraphQL errors (attempt %d): %v\n", attempt+1, errorMessages)
			continue
		}

		// Extract data
		data, ok := rawResponse["data"].(map[string]interface{})
		if !ok {
			lastErr = fmt.Errorf("invalid response structure: missing 'data' field")
			fmt.Printf("DEBUG: Invalid response structure. Response: %s\n", string(bodyBytes))
			continue
		}

		// Try to extract users from datasets (if that's what was queried)
		userSet := make(map[string]bool)
		if datasetsData, ok := data["datasets"].([]interface{}); ok {
			fmt.Printf("DEBUG: Found datasets data, extracting users\n")
			for _, entry := range datasetsData {
				if entryMap, ok := entry.(map[string]interface{}); ok {
					// Try both "owner" (new schema) and "user" (old schema) for backward compatibility
					if owner, ok := entryMap["owner"].(string); ok && owner != "" {
						userSet[owner] = true
					} else if user, ok := entryMap["user"].(string); ok && user != "" {
						userSet[user] = true
					}
				}
			}
		}

		// Also try to extract from events (for backward compatibility)
		if eventsData, ok := data["events"].([]interface{}); ok {
			fmt.Printf("DEBUG: Found events data, extracting users\n")
			for _, event := range eventsData {
				if eventMap, ok := event.(map[string]interface{}); ok {
					if addr, ok := eventMap["account_address"].(string); ok && addr != "" {
						userSet[addr] = true
					}
				}
			}
		}

		users := make([]string, 0, len(userSet))
		for user := range userSet {
			users = append(users, user)
		}

		fmt.Printf("DEBUG: Successfully queried GraphQL indexer, found %d unique users\n", len(users))
		return users, nil
	}

	return nil, fmt.Errorf("GraphQL indexer query failed after 3 attempts: %w", lastErr)
}

// queryUsersFromGraphQLIndexerAlternative queries users by querying account_transactions and filtering events
// This is a fallback when direct events query doesn't work
func (s *AptosServiceImpl) queryUsersFromGraphQLIndexerAlternative(eventType string) ([]string, error) {
	fmt.Printf("DEBUG: Trying alternative approach: query account_transactions with events\n")

	// Query account_transactions and access events within them
	graphQLQuery := `query GetDataSubmittedEvents {
		account_transactions(
			limit: 1000,
			order_by: { transaction_version: desc }
		) {
			account_address
			transaction_version
			events {
				type
				data
			}
		}
	}`

	requestBody := map[string]interface{}{
		"query": graphQLQuery,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", config.AppConfig.AptosIndexerURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "DataX-Backend/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GraphQL request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GraphQL returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var graphQLResponse struct {
		Data struct {
			AccountTransactions []struct {
				AccountAddress     string `json:"account_address"`
				TransactionVersion int64  `json:"transaction_version"`
				Events             []struct {
					Type string          `json:"type"`
					Data json.RawMessage `json:"data"`
				} `json:"events"`
			} `json:"account_transactions"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(bodyBytes, &graphQLResponse); err != nil {
		return nil, fmt.Errorf("failed to decode GraphQL response: %w", err)
	}

	if len(graphQLResponse.Errors) > 0 {
		errorMessages := make([]string, len(graphQLResponse.Errors))
		for i, err := range graphQLResponse.Errors {
			errorMessages[i] = err.Message
		}
		return nil, fmt.Errorf("GraphQL errors: %s", strings.Join(errorMessages, "; "))
	}

	// Filter events by type and extract users
	userSet := make(map[string]bool)
	for _, tx := range graphQLResponse.Data.AccountTransactions {
		for _, event := range tx.Events {
			if event.Type == eventType {
				// Add the account address that emitted the event
				if tx.AccountAddress != "" {
					userSet[tx.AccountAddress] = true
				}

				// Also extract user from event data
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
	}

	users := make([]string, 0, len(userSet))
	for user := range userSet {
		users = append(users, user)
	}

	fmt.Printf("DEBUG: Alternative query found %d unique users\n", len(users))
	return users, nil
}

// discoverUsersFromEventsTable queries recent transactions to find users who called submit_data
// This is a pure blockchain approach - no in-memory storage
func (s *AptosServiceImpl) discoverUsersFromEventsTable() ([]string, error) {
	moduleAddr, err := parseAddress(config.AppConfig.DataXModuleAddr)
	if err != nil {
		return nil, err
	}

	// Query recent transactions and filter for ones that called our submit_data function
	// We'll query transactions and check their payload to see if they call our module
	moduleAddrStr := moduleAddr.String()
	submitDataFunction := fmt.Sprintf("%s::data_registry::submit_data", moduleAddrStr)

	// Query recent transactions from the REST API
	// Query the most recent transactions and filter for ones that called submit_data
	transactionsURL := fmt.Sprintf("%s/v1/transactions?limit=1000", config.AppConfig.AptosNodeURL)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", transactionsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}

	bodyBytes, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()

	if readErr != nil {
		return nil, fmt.Errorf("failed to read response body: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("transactions query returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// The transactions endpoint returns an array directly
	var transactions []map[string]interface{}

	if err := json.Unmarshal(bodyBytes, &transactions); err != nil {
		previewLen := 500
		if len(bodyBytes) < previewLen {
			previewLen = len(bodyBytes)
		}
		fmt.Printf("DEBUG: Failed to decode transactions. Response preview: %s\n", string(bodyBytes[:previewLen]))
		return nil, fmt.Errorf("failed to decode transactions: %w", err)
	}

	fmt.Printf("DEBUG: Retrieved %d transactions from API\n", len(transactions))

	// Extract users from transactions that called our submit_data function
	userSet := make(map[string]bool)
	for i, tx := range transactions {
		// Check transaction type - should be "user_transaction"
		txType, ok := tx["type"].(string)
		if !ok || txType != "user_transaction" {
			continue
		}

		// Get sender
		sender, ok := tx["sender"].(string)
		if !ok || sender == "" {
			continue
		}

		// Get payload - it's nested under "payload" for user_transactions
		payload, ok := tx["payload"].(map[string]interface{})
		if !ok {
			continue
		}

		// Check if it's an entry function payload
		payloadType, ok := payload["type"].(string)
		if !ok || payloadType != "entry_function_payload" {
			continue
		}

		// Get function name
		function, ok := payload["function"].(string)
		if !ok {
			continue
		}

		if function == submitDataFunction {
			userSet[sender] = true
			fmt.Printf("DEBUG: Found user %s from transaction %d calling submit_data\n", sender, i)
		}
	}

	users := make([]string, 0, len(userSet))
	for user := range userSet {
		users = append(users, user)
	}

	fmt.Printf("DEBUG: Discovered %d users from recent transactions\n", len(users))
	return users, nil
}

// queryMarketplaceFromGeomiIndexer queries the indexer's datasets table
func (s *AptosServiceImpl) queryMarketplaceFromGeomiIndexer() ([]interface{}, error) {
	if s.graphqlClient == nil {
		return nil, fmt.Errorf("GraphQL client not initialized")
	}

	apiKey := strings.TrimSpace(config.AppConfig.AptosIndexerAPIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("APTOS_INDEXER_API_KEY is required but not set")
	}

	fmt.Printf("DEBUG: Querying Geomi indexer at: %s\n", config.AppConfig.AptosIndexerURL)
	fmt.Printf("DEBUG: API key present: %v (length: %d chars)\n", apiKey != "", len(apiKey))

	// Use interface{} for dataset_id since it might be string or number
	// Try querying with and without filters to see what works
	var query struct {
		Datasets []struct {
			Owner               string      `graphql:"owner"`
			DataHash            string      `graphql:"data_hash"`
			DatasetID           interface{} `graphql:"dataset_id"`
			IsActive            bool        `graphql:"is_active"`
			Metadata            string      `graphql:"metadata"`
			CreatedAt           uint64      `graphql:"created_at"`
			EncryptionMetadata  string      `graphql:"encryption_metadata"`
			EncryptionAlgorithm string      `graphql:"encryption_algorithm"`
		} `graphql:"datasets"`
	}

	// Try querying all datasets first (no filters)
	// If indexer supports it, we can add filters later like: `graphql:"datasets(where: {is_active: {_eq: true}})"`

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("DEBUG: Executing GraphQL query for datasets...\n")

	// First, try a raw HTTP query to see the actual response
	rawQuery := `query MyQuery {
		datasets {
			data_hash
			dataset_id
			owner
			is_active
			metadata
			created_at
			encryption_metadata
			encryption_algorithm
		}
	}`

	// Try raw HTTP query first for debugging
	requestBody := map[string]interface{}{
		"query": rawQuery,
	}
	jsonBody, _ := json.Marshal(requestBody)

	req, _ := http.NewRequestWithContext(ctx, "POST", config.AppConfig.AptosIndexerURL, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	rawResp, rawErr := s.httpClient.Do(req)
	if rawErr == nil && rawResp != nil {
		rawBodyBytes, _ := io.ReadAll(rawResp.Body)
		rawResp.Body.Close()
		previewLen := 1000
		if len(rawBodyBytes) < previewLen {
			previewLen = len(rawBodyBytes)
		}
		fmt.Printf("DEBUG: Raw GraphQL response (first %d chars): %s\n", previewLen, string(rawBodyBytes[:previewLen]))

		// Try to parse the raw response
		var rawResponse map[string]interface{}
		if json.Unmarshal(rawBodyBytes, &rawResponse) == nil {
			if errors, ok := rawResponse["errors"].([]interface{}); ok && len(errors) > 0 {
				fmt.Printf("DEBUG: GraphQL errors in raw response: %+v\n", errors)
			}
			if data, ok := rawResponse["data"].(map[string]interface{}); ok {
				if datasets, ok := data["datasets"].([]interface{}); ok {
					fmt.Printf("DEBUG: Raw response found %d datasets\n", len(datasets))
				} else {
					fmt.Printf("DEBUG: Raw response data structure: %+v\n", data)
				}
			}
		}
	}

	// Now try the GraphQL client query
	if err := s.graphqlClient.Query(ctx, &query, nil); err != nil {
		fmt.Printf("DEBUG: GraphQL client query error: %v\n", err)
		fmt.Printf("DEBUG: GraphQL query error details - type: %T, error: %+v\n", err, err)
		return nil, fmt.Errorf("GraphQL query failed: %w", err)
	}

	fmt.Printf("DEBUG: GraphQL query succeeded, found %d entries in datasets\n", len(query.Datasets))

	// Log first few entries for debugging
	if len(query.Datasets) > 0 {
		for i, ds := range query.Datasets {
			if i >= 3 {
				break
			}
			fmt.Printf("DEBUG: Dataset %d: owner=%s, data_hash=%s, dataset_id=%v, is_active=%v\n",
				i, ds.Owner, ds.DataHash, ds.DatasetID, ds.IsActive)
		}
	} else {
		fmt.Printf("DEBUG: WARNING - GraphQL query returned 0 datasets. This could mean:\n")
		fmt.Printf("  1. No datasets have been submitted yet\n")
		fmt.Printf("  2. Indexer is still syncing (may take a few seconds after transaction)\n")
		fmt.Printf("  3. Indexer query might need filters or different structure\n")
	}

	// Build initial dataset list from indexer
	indexerDatasets := make([]map[string]interface{}, 0, len(query.Datasets))
	for _, entry := range query.Datasets {
		// Parse dataset_id which might be string or number
		var datasetID uint64
		switch v := entry.DatasetID.(type) {
		case float64:
			datasetID = uint64(v)
		case string:
			parsed, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				fmt.Printf("DEBUG: Failed to parse dataset_id '%v', skipping entry\n", v)
				continue
			}
			datasetID = parsed
		case int64:
			datasetID = uint64(v)
		case int:
			datasetID = uint64(v)
		default:
			fmt.Printf("DEBUG: Unknown dataset_id type %T: %v, skipping entry\n", v, v)
			continue
		}

		indexerDatasets = append(indexerDatasets, map[string]interface{}{
			"id":                   datasetID,
			"owner":                entry.Owner,
			"data_hash":            entry.DataHash,
			"metadata":             entry.Metadata,
			"created_at":           entry.CreatedAt,
			"is_active":            entry.IsActive,
			"encryption_metadata":  entry.EncryptionMetadata,
			"encryption_algorithm": entry.EncryptionAlgorithm,
		})
	}

	fmt.Printf("DEBUG: Converted %d marketplace entries from indexer\n", len(indexerDatasets))

	// Verify is_active status from blockchain for each dataset
	// Even though the indexer now provides is_active, we verify from blockchain for accuracy
	// This ensures we have the most up-to-date status
	fmt.Printf("DEBUG: Verifying is_active status from blockchain for %d datasets...\n", len(indexerDatasets))

	// Use concurrent worker pool to avoid timeouts (max 3 concurrent)
	const maxConcurrent = 3
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	type verifiedDataset struct {
		data     map[string]interface{}
		isActive bool
	}

	resultsChan := make(chan verifiedDataset, len(indexerDatasets))

	for _, ds := range indexerDatasets {
		wg.Add(1)
		go func(dataset map[string]interface{}) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			owner := dataset["owner"].(string)
			datasetID := dataset["id"].(uint64)

			// Query blockchain to get actual is_active status
			datasetInfo, err := s.GetDataset(owner, datasetID)
			if err != nil {
				fmt.Printf("DEBUG: Failed to verify dataset %d for owner %s: %v, skipping\n", datasetID, owner, err)
				return
			}

			// Extract is_active from the returned data
			var isActive bool
			if datasetMap, ok := datasetInfo.(map[string]interface{}); ok {
				if active, ok := datasetMap["is_active"].(bool); ok {
					isActive = active
				}
			}

			// Send result
			resultsChan <- verifiedDataset{data: dataset, isActive: isActive}
		}(ds)
	}

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	datasets := make([]interface{}, 0, len(indexerDatasets))
	for result := range resultsChan {
		if !result.isActive {
			datasetID := result.data["id"].(uint64)
			owner := result.data["owner"].(string)
			fmt.Printf("DEBUG: Dataset %d from owner %s is inactive (deleted), excluding from marketplace\n", datasetID, owner)
			continue
		}

		// Add is_active to the dataset
		result.data["is_active"] = true
		datasets = append(datasets, result.data)
	}

	fmt.Printf("DEBUG: After filtering deleted datasets: %d active datasets (from %d indexed)\n", len(datasets), len(indexerDatasets))
	return datasets, nil
}

// GetMarketplaceDatasets returns all datasets from the marketplace
// Uses indexer to fetch data from datasets table, with blockchain fallback
// It discovers users from chain events and queries their DataStore resources to get all datasets
// This approach fetches data directly from on-chain state, not from memory
func (s *AptosServiceImpl) GetMarketplaceDatasets() ([]interface{}, error) {
	fmt.Printf("DEBUG: GetMarketplaceDatasets endpoint called\n")

	// Check if indexer is configured
	if config.AppConfig.AptosIndexerURL == "" {
		fmt.Printf("DEBUG: Indexer URL not configured, falling back to blockchain query\n")
		return s.getMarketplaceDatasetsFromBlockchain()
	}

	// Try to query from Geomi indexer first
	fmt.Printf("DEBUG: Attempting to query Geomi indexer for marketplace data...\n")
	datasets, err := s.queryMarketplaceFromGeomiIndexer()
	if err != nil {
		fmt.Printf("DEBUG: Failed to query Geomi indexer: %v\n", err)
		fmt.Printf("DEBUG: Falling back to blockchain query method...\n")
		return s.getMarketplaceDatasetsFromBlockchain()
	}

	fmt.Printf("DEBUG: Successfully queried Geomi indexer, found %d datasets\n", len(datasets))

	// If indexer returns 0 datasets, it might be empty OR it might be out of sync/broken
	// So we should fall back to blockchain query just in case
	if len(datasets) == 0 {
		fmt.Printf("DEBUG: No datasets found in indexer, falling back to blockchain query to be sure\n")
		return s.getMarketplaceDatasetsFromBlockchain()
	}

	fmt.Printf("DEBUG: GetMarketplaceDatasets completed, returning %d datasets\n", len(datasets))
	return datasets, nil
}

// getMarketplaceDatasetsFromBlockchain is the fallback method that queries blockchain directly
func (s *AptosServiceImpl) getMarketplaceDatasetsFromBlockchain() ([]interface{}, error) {
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

	// Fallback: If no users found via events, try to discover by querying events table directly
	// This is a more reliable approach for the Aptos indexer
	if len(users) == 0 {
		fmt.Printf("DEBUG: No users found via DiscoverUsersFromChain, trying direct events query...\n")
		users, err = s.discoverUsersFromEventsTable()
		if err != nil {
			fmt.Printf("DEBUG: Error discovering users from events table: %v\n", err)
		} else {
			fmt.Printf("DEBUG: Discovered %d users from events table\n", len(users))
		}
	}

	// No registry - all users come from blockchain discovery
	fmt.Printf("DEBUG: Total users to query: %d (all from blockchain)\n", len(users))

	if len(users) == 0 {
		fmt.Printf("DEBUG: No users found. Datasets may not exist yet, or indexer is not working properly.\n")
		fmt.Printf("DEBUG: Consider checking:\n")
		fmt.Printf("DEBUG: 1. USE_INDEXER environment variable (should be true)\n")
		fmt.Printf("DEBUG: 2. APTOS_INDEXER_URL is set correctly\n")
		fmt.Printf("DEBUG: 3. There are actual DataSubmitted events on-chain\n")
		return []interface{}{}, nil
	}

	// Step 3: Query DataStore resources directly from each discovered user account
	// This is more reliable than querying events, as it gets data directly from on-chain state
	// Use concurrent requests with proper error handling
	datasets := make([]interface{}, 0)
	seenDatasets := make(map[string]bool) // Track owner+datasetID to avoid duplicates
	datasetsMutex := sync.Mutex{}         // Protect datasets slice

	resourceType := fmt.Sprintf("%s::data_registry::DataStore", moduleAddr.String())

	// Use a worker pool to query users concurrently (max 3 concurrent requests to avoid overwhelming the API)
	const maxConcurrent = 3
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
			var bodyBytes []byte

			// Retry up to 2 times
			for attempt := 0; attempt < 2; attempt++ {
				if attempt > 0 {
					time.Sleep(500 * time.Millisecond)
				}

				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				req, reqErr := http.NewRequestWithContext(ctx, "GET", resourceURL, nil)
				if reqErr != nil {
					cancel()
					err = reqErr
					continue
				}

				resp, err = s.httpClient.Do(req)

				if err != nil {
					cancel()
					if resp != nil {
						resp.Body.Close()
					}
					fmt.Printf("DEBUG: Request failed for %s (attempt %d): %v\n", addr, attempt+1, err)
					continue
				}

				if resp.StatusCode == http.StatusNotFound {
					cancel()
					resp.Body.Close()
					fmt.Printf("DEBUG: No DataStore found for user %s\n", addr)
					return
				}

				if resp.StatusCode != http.StatusOK {
					cancel()
					resp.Body.Close()
					fmt.Printf("DEBUG: DataStore query returned status %d for user %s\n", resp.StatusCode, addr)
					return
				}

				// Read the entire response body before canceling context
				bodyBytes, err = io.ReadAll(resp.Body)
				resp.Body.Close()
				cancel()

				if err != nil {
					fmt.Printf("DEBUG: Failed to read response body for %s (attempt %d): %v\n", addr, attempt+1, err)
					continue
				}

				// Success - break out of retry loop
				break
			}

			if err != nil || bodyBytes == nil {
				fmt.Printf("DEBUG: Failed to query DataStore from %s after retries: %v\n", addr, err)
				return
			}

			// Parse the DataStore resource from the already-read body bytes
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

			if err := json.Unmarshal(bodyBytes, &resourceData); err != nil {
				fmt.Printf("DEBUG: Failed to decode DataStore from %s: %v\n", addr, err)
				fmt.Printf("DEBUG: Response body length: %d bytes\n", len(bodyBytes))
				if len(bodyBytes) > 0 && len(bodyBytes) < 500 {
					fmt.Printf("DEBUG: Response body preview: %s\n", string(bodyBytes))
				}
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

// RequestAccess stores an access request
// Note: In a production system, access requests should be stored on-chain
// For now, this is a no-op as we're removing in-memory storage
func RequestAccess(ownerAddress string, datasetID uint64, requesterAddress string, message string) {
	// Access requests should be stored on-chain via a smart contract
	// This function is kept for API compatibility but does nothing
	fmt.Printf("DEBUG: Access request received (not stored - should be on-chain): owner=%s, dataset=%d, requester=%s\n",
		ownerAddress, datasetID, requesterAddress)
}

// GetAccessRequests returns access requests for a dataset owner
// Note: In a production system, this should query on-chain access requests
func (s *AptosServiceImpl) GetAccessRequests(ownerAddress string) ([]interface{}, error) {
	// Access requests should be queried from the blockchain
	// For now, return empty list as we're removing in-memory storage
	fmt.Printf("DEBUG: GetAccessRequests called for %s (returning empty - should query blockchain)\n", ownerAddress)
	return []interface{}{}, nil
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

// GetUserDatasetsMetadata returns minimal metadata (id, metadata, is_active) for all datasets
// This is optimized for batch operations like populating dropdowns
func (s *AptosServiceImpl) GetUserDatasetsMetadata(userAddress string) ([]interface{}, error) {
	userAddr, err := parseAddress(userAddress)
	if err != nil {
		return nil, err
	}

	moduleAddr, err := parseAddress(config.AppConfig.DataXModuleAddr)
	if err != nil {
		return nil, err
	}

	// Query the DataStore resource directly
	resourceType := fmt.Sprintf("%s::data_registry::DataStore", moduleAddr.String())
	resourceURL := fmt.Sprintf("%s/v1/accounts/%s/resource/%s",
		config.AppConfig.AptosNodeURL,
		userAddr.String(),
		url.PathEscape(resourceType))

	// Retry logic with exponential backoff
	var resp *http.Response
	var bodyBytes []byte
	var lastErr error
	var lastStatusCode int

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			fmt.Printf("DEBUG: Retrying GetUserDatasetsMetadata query (attempt %d/3) after %v\n", attempt+1, backoff)
			time.Sleep(backoff)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", resourceURL, nil)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}

		resp, err = s.httpClient.Do(req)
		cancel()

		if err != nil {
			lastErr = fmt.Errorf("failed to query DataStore resource: %w", err)
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}

		bodyBytes, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		lastStatusCode = resp.StatusCode

		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			bodyBytes = nil
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			// No DataStore resource - return empty array
			return []interface{}{}, nil
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("rate limited (429)")
			bodyBytes = nil
			if attempt < 2 {
				time.Sleep(5 * time.Second)
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			bodyBytes = nil
			if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
				return nil, lastErr
			}
			continue
		}

		// Success
		break
	}

	if resp == nil {
		return nil, fmt.Errorf("failed to query DataStore resource after retries: %w", lastErr)
	}

	if lastStatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to query DataStore resource: status %d, error: %w", lastStatusCode, lastErr)
	}

	if len(bodyBytes) == 0 {
		return []interface{}{}, nil
	}

	// Parse the resource data
	var resourceData struct {
		Data struct {
			Datasets []struct {
				ID       interface{} `json:"id"`
				Metadata interface{} `json:"metadata"`
				IsActive interface{} `json:"is_active"`
			} `json:"datasets"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &resourceData); err != nil {
		return nil, fmt.Errorf("failed to decode resource data: %w", err)
	}

	// Convert to minimal metadata format
	result := make([]interface{}, 0, len(resourceData.Data.Datasets))
	for _, dataset := range resourceData.Data.Datasets {
		// Parse ID
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

		// Parse metadata
		metadataStr := ""
		switch v := dataset.Metadata.(type) {
		case []interface{}:
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
			metadataStr = v
		default:
			metadataStr = fmt.Sprintf("%v", v)
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

		result = append(result, map[string]interface{}{
			"id":        id,
			"metadata":  metadataStr,
			"is_active": isActive,
		})
	}

	return result, nil
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

// CheckDataHashExists checks if a data hash already exists in the marketplace
func (s *AptosServiceImpl) CheckDataHashExists(dataHash string) (bool, error) {
	// Ensure hash format (0x prefix)
	if !strings.HasPrefix(dataHash, "0x") {
		dataHash = "0x" + dataHash
	}

	// 1. Try Indexer first (most efficient)
	if config.AppConfig.AptosIndexerURL != "" {
		exists, err := s.checkDataHashFromIndexer(dataHash)
		if err == nil && exists {
			// If indexer says it exists, it definitely exists
			return true, nil
		}
		// If indexer says false, it might be lagging, so we fall back to blockchain
		if err != nil {
			fmt.Printf("DEBUG: Indexer check failed: %v. Falling back to blockchain.\n", err)
		} else {
			fmt.Printf("DEBUG: Indexer returned false, double-checking with blockchain (in case of lag).\n")
		}
	}

	// 2. Fallback: Get all datasets and check (less efficient but reliable)
	datasets, err := s.GetMarketplaceDatasets()
	if err != nil {
		return false, err
	}

	for _, d := range datasets {
		if datasetMap, ok := d.(map[string]interface{}); ok {
			if hash, ok := datasetMap["data_hash"].(string); ok {
				if hash == dataHash {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func (s *AptosServiceImpl) checkDataHashFromIndexer(dataHash string) (bool, error) {
	if s.graphqlClient == nil {
		return false, fmt.Errorf("GraphQL client not initialized")
	}

	var query struct {
		Datasets []struct {
			DataHash string `graphql:"data_hash"`
		} `graphql:"datasets(where: {data_hash: {_eq: $data_hash}})"`
	}

	variables := map[string]interface{}{
		"data_hash": dataHash,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.graphqlClient.Query(ctx, &query, variables); err != nil {
		return false, err
	}

	return len(query.Datasets) > 0, nil
}
