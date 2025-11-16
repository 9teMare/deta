package services

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/datax/backend/config"
)

type StorageService interface {
	StoreCSV(accountAddress string, data [][]string) (string, error)
	RetrieveCSV(accountAddress string, blobName string) ([][]string, error)
}

type ShelbyServiceImpl struct {
	rpcURL     string
	accountKey string
	httpClient *http.Client
}

func NewShelbyService() StorageService {
	rpcURL := config.AppConfig.ShelbyRPCURL
	if rpcURL == "" {
		// Default to devnet RPC
		rpcURL = "https://rpc.shelby.xyz"
	}

	// Remove trailing slash
	rpcURL = strings.TrimSuffix(rpcURL, "/")

	return &ShelbyServiceImpl{
		rpcURL:     rpcURL,
		accountKey: config.AppConfig.ShelbyAccountKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// createMicropaymentChannel creates a micropayment channel session for the account
// According to Shelby API: POST /v1/sessions/micropaymentchannels
func (s *ShelbyServiceImpl) createMicropaymentChannel(accountAddress string) error {
	sessionURL := fmt.Sprintf("%s/v1/sessions/micropaymentchannels", s.rpcURL)

	// Create request body
	reqBody := map[string]interface{}{
		"account": accountAddress,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal session request: %w", err)
	}

	req, err := http.NewRequest("POST", sessionURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create session request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if s.accountKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.accountKey)
	}

	fmt.Printf("DEBUG: Creating Shelby micropayment channel: URL=%s\n", sessionURL)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		fmt.Printf("ERROR: Shelby session creation failed: %v\n", err)
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	fmt.Printf("DEBUG: Shelby session response: Status=%d, Body=%s\n", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Session might already exist, which is okay
		if resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusBadRequest {
			fmt.Printf("DEBUG: Session may already exist (status %d), continuing...\n", resp.StatusCode)
			return nil
		}
		return fmt.Errorf("shelby session creation failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	fmt.Printf("DEBUG: Successfully created Shelby micropayment channel\n")
	return nil
}

// StoreCSV stores CSV data on Shelby and returns the blob name
// According to Shelby API: POST /v1/blobs/{account}/{blobName}
func (s *ShelbyServiceImpl) StoreCSV(accountAddress string, data [][]string) (string, error) {
	// First, create a micropayment channel session
	if err := s.createMicropaymentChannel(accountAddress); err != nil {
		return "", fmt.Errorf("failed to create session before upload: %w", err)
	}

	// Convert CSV to bytes
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	for _, row := range data {
		if err := writer.Write(row); err != nil {
			return "", fmt.Errorf("failed to write CSV row: %w", err)
		}
	}
	writer.Flush()

	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("failed to flush CSV: %w", err)
	}

	csvBytes := buf.Bytes()

	// Generate a unique blob name based on content hash
	// In production, you might want to use a more sophisticated naming scheme
	blobName := fmt.Sprintf("csv_%d_%x", time.Now().Unix(), csvBytes[:min(16, len(csvBytes))])

	// Upload to Shelby API
	// Shelby API: POST /v1/blobs/{account}/{blobName}
	// Account address should be in the path
	uploadURL := fmt.Sprintf("%s/v1/blobs/%s/%s", s.rpcURL, accountAddress, blobName)

	req, err := http.NewRequest("POST", uploadURL, bytes.NewReader(csvBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Content-Type", "text/csv")
	if s.accountKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.accountKey)
	}

	fmt.Printf("DEBUG: Uploading CSV to Shelby: URL=%s, Size=%d bytes\n", uploadURL, len(csvBytes))
	resp, err := s.httpClient.Do(req)
	if err != nil {
		fmt.Printf("ERROR: Shelby upload request failed: %v\n", err)
		return "", fmt.Errorf("failed to upload to Shelby: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	fmt.Printf("DEBUG: Shelby upload response: Status=%d, Body=%s\n", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("shelby upload failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response to get blob identifier
	// Note: We need to re-read the body since we already read it for error checking
	// But we already read it above, so we'll use the bodyBytes we captured
	var uploadResp struct {
		BlobName   string `json:"blob_name"`
		MerkleRoot string `json:"merkle_root,omitempty"`
	}

	if err := json.Unmarshal(bodyBytes, &uploadResp); err != nil {
		// If response is not JSON, use the blob name we generated
		fmt.Printf("DEBUG: Shelby response is not JSON, using generated blob name: %s\n", blobName)
		return blobName, nil
	}

	if uploadResp.BlobName != "" {
		fmt.Printf("DEBUG: Shelby returned blob name: %s\n", uploadResp.BlobName)
		return uploadResp.BlobName, nil
	}

	fmt.Printf("DEBUG: Using generated blob name: %s\n", blobName)
	return blobName, nil
}

// RetrieveCSV retrieves CSV data from Shelby using blob name
// According to Shelby API: GET /v1/blobs/{account}/{blobName}
func (s *ShelbyServiceImpl) RetrieveCSV(accountAddress string, blobName string) ([][]string, error) {
	// Download from Shelby API
	// Shelby API: GET /v1/blobs/{account}/{blobName}
	// Account address should be in the path
	downloadURL := fmt.Sprintf("%s/v1/blobs/%s/%s", s.rpcURL, accountAddress, blobName)

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	if s.accountKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.accountKey)
	}

	fmt.Printf("DEBUG: Downloading CSV from Shelby: URL=%s\n", downloadURL)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		fmt.Printf("ERROR: Shelby download request failed: %v\n", err)
		return nil, fmt.Errorf("failed to download from Shelby: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	fmt.Printf("DEBUG: Shelby download response: Status=%d, Body length=%d\n", resp.StatusCode, len(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("shelby download failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Use the body bytes we already read
	data := bodyBytes

	// Parse CSV
	csvReader := csv.NewReader(bytes.NewReader(data))
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	return records, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
