package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/datax/backend/config"
)

type StorageService interface {
	StoreCSV(accountAddress string, data [][]string) (string, error)
	RetrieveCSV(accountAddress string, blobName string) ([][]string, error)
	// StoreEncryptedCSV stores encrypted CSV data and returns blob name and encryption metadata
	StoreEncryptedCSV(accountAddress string, encryptedData []byte, encryptionMetadata string) (string, error)
	// RetrieveEncryptedCSV retrieves encrypted CSV data
	RetrieveEncryptedCSV(accountAddress string, blobName string) ([]byte, string, error)
}

type SupabaseServiceImpl struct {
	s3Client          *s3.Client
	bucketName        string
	encryptionService *EncryptionService
}

func NewSupabaseService() StorageService {
	s3URL := config.AppConfig.SupabaseS3URL
	supabaseKey := config.AppConfig.SupabaseKey
	accessKey := config.AppConfig.SupabaseAccessKey
	secretKey := config.AppConfig.SupabaseSecretKey

	if s3URL == "" {
		panic("SUPABASE_S3_URL is not set")
	}

	// Parse the endpoint URL
	endpoint := strings.TrimSuffix(s3URL, "/")

	var accessKeyId, secretAccessKey string

	// Check if actual S3 credentials are provided
	if accessKey != "" && secretKey != "" {
		// Use actual S3 credentials
		accessKeyId = accessKey
		secretAccessKey = secretKey
		fmt.Printf("DEBUG: Using provided S3 credentials (access key first 10 chars): %s...\n", accessKey[:minInt(10, len(accessKey))])
	} else if supabaseKey != "" {
		// Fallback to project_ref + anon key approach
		projectRef := extractProjectRef(s3URL)
		accessKeyId = projectRef
		secretAccessKey = supabaseKey
		fmt.Printf("DEBUG: Using project_ref + anon key approach (project_ref: %s)\n", projectRef)
		keyPreview := supabaseKey
		if len(keyPreview) > 10 {
			keyPreview = keyPreview[:10]
		}
		fmt.Printf("DEBUG: Using Supabase key (first 10 chars): %s...\n", keyPreview)
	} else {
		panic("Either SUPABASE_ACCESS_KEY + SUPABASE_SECRET_KEY or SUPABASE_KEY must be set")
	}

	// Create AWS config with custom credentials and endpoint
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion("us-east-1"), // Dummy region, Supabase doesn't use it
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKeyId,
			secretAccessKey,
			"", // sessionToken (not needed for backend)
		)),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to load AWS config: %v", err))
	}

	// Create S3 client with custom endpoint and forcePathStyle
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true // forcePathStyle: true (required for Supabase)
	})

	return &SupabaseServiceImpl{
		s3Client:          s3Client,
		bucketName:        config.AppConfig.SupabaseBucket,
		encryptionService: NewEncryptionService(),
	}
}

// extractProjectRef extracts the project reference from Supabase S3 URL
// URL format: https://project_ref.storage.supabase.co/storage/v1/s3
func extractProjectRef(url string) string {
	// Remove protocol
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Extract project_ref from domain
	// Format: project_ref.storage.supabase.co/storage/v1/s3
	parts := strings.Split(url, ".")
	if len(parts) > 0 {
		projectRef := parts[0]
		if projectRef != "" {
			return projectRef
		}
	}

	// Alternative: try to extract from full URL pattern
	// https://ufyhlybeyizfbfkgbbil.storage.supabase.co/storage/v1/s3
	if strings.Contains(url, ".storage.supabase.co") {
		storageIndex := strings.Index(url, ".storage.supabase.co")
		if storageIndex > 0 {
			return url[:storageIndex]
		}
	}

	// Fallback: try to extract from path if domain format is different
	if strings.Contains(url, "/storage/v1/s3") {
		// If URL is already just the path, extract from a different format
		return "project_ref" // This shouldn't happen, but provide fallback
	}

	panic(fmt.Sprintf("Failed to extract project_ref from URL: %s", url))
}

// StoreCSV stores CSV data in Supabase Storage (S3-compatible) and returns the blob name/path
// NOTE: This method now encrypts data before storing. Use StoreEncryptedCSV for pre-encrypted data.
func (s *SupabaseServiceImpl) StoreCSV(accountAddress string, data [][]string) (string, error) {
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

	// Derive encryption key from account address (deterministic)
	// In production, you might want to use a user-provided key or key from wallet
	encryptionKey := DeriveKeyFromAddress(accountAddress, []byte("datax-salt-v1"))

	// Encrypt the CSV data
	encryptedData, encryptionMetadata, err := s.encryptionService.EncryptCSVData(csvBytes, encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt CSV data: %w", err)
	}

	// Generate a unique blob name based on account and timestamp
	// Format: {account}/{timestamp}_{hash}.csv.enc
	timestamp := time.Now().Unix()
	hashLen := 16
	if len(encryptedData) < hashLen {
		hashLen = len(encryptedData)
	}
	hash := fmt.Sprintf("%x", encryptedData[:hashLen])
	blobName := fmt.Sprintf("%s/%d_%s.csv.enc", accountAddress, timestamp, hash)

	// Store encryption metadata in a separate metadata file
	metadataBlobName := fmt.Sprintf("%s.meta", blobName)
	metadataBytes := []byte(encryptionMetadata)

	ctx := context.Background()

	// Upload encrypted data
	_, err = s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(blobName),
		Body:        bytes.NewReader(encryptedData),
		ContentType: aws.String("application/octet-stream"), // Encrypted data
	})
	if err != nil {
		fmt.Printf("ERROR: Supabase S3 upload failed: %v\n", err)
		return "", fmt.Errorf("failed to upload encrypted data to Supabase S3: %w", err)
	}

	// Upload encryption metadata
	_, err = s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(metadataBlobName),
		Body:        bytes.NewReader(metadataBytes),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		fmt.Printf("WARNING: Failed to upload encryption metadata: %v\n", err)
		// Continue anyway, metadata might be stored on-chain
	}

	fmt.Printf("DEBUG: Successfully stored encrypted CSV in Supabase Storage with path: %s\n", blobName)
	return blobName, nil
}

// StoreEncryptedCSV stores pre-encrypted CSV data (for client-side encryption)
func (s *SupabaseServiceImpl) StoreEncryptedCSV(accountAddress string, encryptedData []byte, encryptionMetadata string) (string, error) {
	// Generate a unique blob name based on account and timestamp
	timestamp := time.Now().Unix()
	hashLen := 16
	if len(encryptedData) < hashLen {
		hashLen = len(encryptedData)
	}
	hash := fmt.Sprintf("%x", encryptedData[:hashLen])
	blobName := fmt.Sprintf("%s/%d_%s.csv.enc", accountAddress, timestamp, hash)

	// Store encryption metadata in a separate metadata file
	metadataBlobName := fmt.Sprintf("%s.meta", blobName)
	metadataBytes := []byte(encryptionMetadata)

	ctx := context.Background()

	// Upload encrypted data
	_, err := s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(blobName),
		Body:        bytes.NewReader(encryptedData),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload encrypted data: %w", err)
	}

	// Upload encryption metadata
	_, err = s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(metadataBlobName),
		Body:        bytes.NewReader(metadataBytes),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		fmt.Printf("WARNING: Failed to upload encryption metadata: %v\n", err)
	}

	return blobName, nil
}

// ListCSVFiles lists all CSV files for an account (used for finding files when mapping is lost)
func (s *SupabaseServiceImpl) ListCSVFiles(accountAddress string) ([]string, error) {
	ctx := context.Background()

	// List objects with prefix: {accountAddress}/
	prefix := accountAddress + "/"

	fmt.Printf("DEBUG: Listing CSV files for account %s with prefix: %s\n", accountAddress, prefix)

	result, err := s.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		fmt.Printf("ERROR: Failed to list objects: %v\n", err)
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	var keys []string
	for _, obj := range result.Contents {
		if strings.HasSuffix(*obj.Key, ".csv") {
			keys = append(keys, *obj.Key)
		}
	}

	fmt.Printf("DEBUG: Found %d CSV files for account %s\n", len(keys), accountAddress)
	return keys, nil
}

// RetrieveCSV retrieves CSV data from Supabase Storage (S3-compatible) using blob name/path
// NOTE: This method now decrypts data after retrieval. Use RetrieveEncryptedCSV for raw encrypted data.
func (s *SupabaseServiceImpl) RetrieveCSV(accountAddress string, blobName string) ([][]string, error) {
	// Try to retrieve encrypted data first
	encryptedData, metadata, err := s.RetrieveEncryptedCSV(accountAddress, blobName)
	if err != nil {
		// If encrypted retrieval fails, try old format (unencrypted) for backward compatibility
		fmt.Printf("DEBUG: Encrypted retrieval failed, trying unencrypted format: %v\n", err)
		return s.retrieveUnencryptedCSV(accountAddress, blobName)
	}

	// Derive encryption key from account address (same as encryption)
	encryptionKey := DeriveKeyFromAddress(accountAddress, []byte("datax-salt-v1"))

	// Decrypt the data
	csvBytes, err := s.encryptionService.DecryptCSVData(encryptedData, encryptionKey, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt CSV data: %w", err)
	}

	// Parse CSV
	csvReader := csv.NewReader(bytes.NewReader(csvBytes))
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	fmt.Printf("DEBUG: Successfully retrieved and decrypted CSV from Supabase Storage: %d rows\n", len(records))
	return records, nil
}

// RetrieveEncryptedCSV retrieves encrypted CSV data and metadata
func (s *SupabaseServiceImpl) RetrieveEncryptedCSV(accountAddress string, blobName string) ([]byte, string, error) {
	ctx := context.Background()

	// Handle different blob name formats
	key := blobName
	if !strings.Contains(blobName, "/") {
		key = fmt.Sprintf("%s/%s", accountAddress, blobName)
	}

	// Try with .enc extension if not present
	if !strings.HasSuffix(key, ".enc") && !strings.HasSuffix(key, ".csv") {
		key = key + ".enc"
	} else if strings.HasSuffix(key, ".csv") {
		// Replace .csv with .enc
		key = strings.TrimSuffix(key, ".csv") + ".enc"
	}

	// Download encrypted data
	result, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		// Try without .enc extension
		if strings.HasSuffix(key, ".enc") {
			keyWithoutEnc := strings.TrimSuffix(key, ".enc")
			result, err = s.s3Client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(s.bucketName),
				Key:    aws.String(keyWithoutEnc),
			})
		}
		if err != nil {
			return nil, "", fmt.Errorf("failed to download encrypted data: %w", err)
		}
	}
	defer result.Body.Close()

	encryptedData, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read encrypted data: %w", err)
	}

	// Download encryption metadata
	metadataKey := fmt.Sprintf("%s.meta", key)
	metadataResult, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(metadataKey),
	})
	if err != nil {
		// Try alternative metadata key
		metadataKey = fmt.Sprintf("%s.meta", strings.TrimSuffix(key, ".enc"))
		metadataResult, err = s.s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s.bucketName),
			Key:    aws.String(metadataKey),
		})
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to download encryption metadata: %w", err)
	}
	defer metadataResult.Body.Close()

	metadataBytes, err := io.ReadAll(metadataResult.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read encryption metadata: %w", err)
	}

	return encryptedData, string(metadataBytes), nil
}

// retrieveUnencryptedCSV retrieves unencrypted CSV (for backward compatibility)
func (s *SupabaseServiceImpl) retrieveUnencryptedCSV(accountAddress string, blobName string) ([][]string, error) {
	ctx := context.Background()

	key := blobName
	if !strings.Contains(blobName, "/") {
		key = fmt.Sprintf("%s/%s", accountAddress, blobName)
	}

	result, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download from Supabase S3: %w", err)
	}
	defer result.Body.Close()

	bodyBytes, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read S3 data: %w", err)
	}

	csvReader := csv.NewReader(bytes.NewReader(bodyBytes))
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	return records, nil
}

// FindBlobByPattern tries to find a blob by listing objects with a prefix pattern
// This is a fallback when the mapping is missing
func (s *SupabaseServiceImpl) FindBlobByPattern(accountAddress string, pattern string) (string, error) {
	ctx := context.Background()

	// List objects with prefix: {account}/
	prefix := accountAddress + "/"

	fmt.Printf("DEBUG: Searching for blob with prefix: %s, pattern: %s\n", prefix, pattern)

	listInput := &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucketName),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(100),
	}

	result, err := s.s3Client.ListObjectsV2(ctx, listInput)
	if err != nil {
		return "", fmt.Errorf("failed to list objects: %w", err)
	}

	fmt.Printf("DEBUG: Found %d objects with prefix %s\n", len(result.Contents), prefix)

	// If no objects found with account prefix, try listing all CSV files in bucket
	// (in case files are stored without account prefix, e.g., just {timestamp}_{hash}.csv)
	if len(result.Contents) == 0 {
		fmt.Printf("DEBUG: No objects found with prefix %s, trying to list all CSV files in bucket\n", prefix)
		allObjectsInput := &s3.ListObjectsV2Input{
			Bucket:  aws.String(s.bucketName),
			MaxKeys: aws.Int32(100),
		}
		allResult, err := s.s3Client.ListObjectsV2(ctx, allObjectsInput)
		if err == nil && len(allResult.Contents) > 0 {
			// Filter for CSV files
			var csvFiles []s3Types.Object
			for _, obj := range allResult.Contents {
				if obj.Key != nil && strings.HasSuffix(*obj.Key, ".csv") {
					csvFiles = append(csvFiles, obj)
				}
			}
			if len(csvFiles) > 0 {
				// Return the most recent CSV file
				var latestObj *s3Types.Object
				for i := range csvFiles {
					obj := &csvFiles[i]
					if latestObj == nil || (obj.LastModified != nil && latestObj.LastModified != nil && obj.LastModified.After(*latestObj.LastModified)) {
						latestObj = obj
					}
				}
				if latestObj != nil && latestObj.Key != nil {
					fmt.Printf("DEBUG: Found CSV file without account prefix: %s\n", *latestObj.Key)
					return *latestObj.Key, nil
				}
			}
		}
		return "", fmt.Errorf("no objects found with prefix: %s", prefix)
	}

	// If pattern is empty, return the most recent CSV object (by LastModified)
	if pattern == "" {
		var latestObj *s3Types.Object
		for _, obj := range result.Contents {
			if obj.Key != nil && strings.HasSuffix(*obj.Key, ".csv") {
				if latestObj == nil || (obj.LastModified != nil && latestObj.LastModified != nil && obj.LastModified.After(*latestObj.LastModified)) {
					latestObj = &obj
				}
			}
		}
		if latestObj != nil && latestObj.Key != nil {
			fmt.Printf("DEBUG: Returning most recent CSV object: %s\n", *latestObj.Key)
			return *latestObj.Key, nil
		}
	}

	// Try to find matching object by pattern
	// The blob name format is: {account}/{timestamp}_{hash}.csv
	// We'll match by the hash pattern in the filename
	for _, obj := range result.Contents {
		if obj.Key != nil {
			key := *obj.Key
			fmt.Printf("DEBUG: Checking object: %s\n", key)
			// Check if key contains the pattern (hash part of filename)
			if pattern != "" && strings.Contains(key, pattern) {
				fmt.Printf("DEBUG: Found matching object: %s\n", key)
				return key, nil
			}
			// Also check if the filename matches the pattern (without account prefix)
			filename := key
			if strings.Contains(key, "/") {
				filename = key[strings.LastIndex(key, "/")+1:]
			}
			if pattern != "" && strings.Contains(filename, pattern) {
				fmt.Printf("DEBUG: Found matching object by filename: %s\n", key)
				return key, nil
			}
		}
	}

	// If no pattern match but we have objects, return the most recent one
	if len(result.Contents) > 0 {
		var latestObj *s3Types.Object
		for _, obj := range result.Contents {
			if latestObj == nil || (obj.LastModified != nil && latestObj.LastModified != nil && obj.LastModified.After(*latestObj.LastModified)) {
				latestObj = &obj
			}
		}
		if latestObj != nil && latestObj.Key != nil {
			fmt.Printf("DEBUG: No pattern match, returning most recent object: %s\n", *latestObj.Key)
			return *latestObj.Key, nil
		}
	}

	return "", fmt.Errorf("no matching blob found with pattern: %s", pattern)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
