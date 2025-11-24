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

type SupabaseServiceImpl struct {
	s3Client   *s3.Client
	bucketName string
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
		s3Client:   s3Client,
		bucketName: config.AppConfig.SupabaseBucket,
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

	// Generate a unique blob name based on account and timestamp
	// Format: {account}/{timestamp}_{hash}.csv
	timestamp := time.Now().Unix()
	hashLen := 16
	if len(csvBytes) < hashLen {
		hashLen = len(csvBytes)
	}
	hash := fmt.Sprintf("%x", csvBytes[:hashLen])
	blobName := fmt.Sprintf("%s/%d_%s.csv", accountAddress, timestamp, hash)

	// Upload to S3 using PutObject
	ctx := context.Background()
	_, err := s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(blobName),
		Body:        bytes.NewReader(csvBytes),
		ContentType: aws.String("text/csv"),
	})

	if err != nil {
		fmt.Printf("ERROR: Supabase S3 upload failed: %v\n", err)
		return "", fmt.Errorf("failed to upload to Supabase S3: %w", err)
	}

	fmt.Printf("DEBUG: Successfully stored CSV in Supabase Storage with path: %s\n", blobName)
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
func (s *SupabaseServiceImpl) RetrieveCSV(accountAddress string, blobName string) ([][]string, error) {
	ctx := context.Background()

	// The blobName might be in different formats:
	// 1. Full path: {account}/{timestamp}_{hash}.csv
	// 2. Just filename: {timestamp}_{hash}.csv (missing account prefix)
	// 3. Just filename without account: {timestamp}_{hash}.csv
	key := blobName

	// If blobName doesn't contain "/", it might be missing the account prefix
	if !strings.Contains(blobName, "/") {
		// Try with account prefix first
		key = fmt.Sprintf("%s/%s", accountAddress, blobName)
		fmt.Printf("DEBUG: Blob name missing account prefix, trying with prefix: %s\n", key)
	} else {
		// Check if the prefix matches the account address
		parts := strings.Split(blobName, "/")
		if len(parts) > 0 && parts[0] != accountAddress {
			// The prefix doesn't match, but try it anyway (might be correct)
			fmt.Printf("DEBUG: Blob name prefix (%s) doesn't match account (%s), using as-is\n", parts[0], accountAddress)
		}
	}

	fmt.Printf("DEBUG: Retrieving CSV from Supabase S3: bucket=%s, key=%s\n", s.bucketName, key)

	// Download from S3 using GetObject
	// Try with the constructed key first
	result, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		// If failed and we added the account prefix, try without it
		if !strings.Contains(blobName, "/") && strings.Contains(key, "/") {
			// Try the original blobName without prefix
			fmt.Printf("DEBUG: Failed with account prefix, trying without prefix: %s\n", blobName)
			result, err = s.s3Client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(s.bucketName),
				Key:    aws.String(blobName),
			})
		}
		if err != nil {
			fmt.Printf("ERROR: Supabase S3 download failed: %v\n", err)
			return nil, fmt.Errorf("failed to download from Supabase S3: %w", err)
		}
	}
	defer result.Body.Close()

	// Read CSV data
	bodyBytes, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read S3 data: %w", err)
	}

	fmt.Printf("DEBUG: Supabase download response: Body length=%d\n", len(bodyBytes))

	// Parse CSV
	csvReader := csv.NewReader(bytes.NewReader(bodyBytes))
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	fmt.Printf("DEBUG: Successfully retrieved CSV from Supabase Storage: %d rows\n", len(records))
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

// Access Request Management Methods
// These methods interact with Supabase database (not S3) for managing access requests

// Note: For database operations, we'll use HTTP requests to Supabase REST API
// since we're already using S3 client for storage

func (s *SupabaseServiceImpl) CreateAccessRequest(ownerAddress, requesterAddress string, datasetID uint64, message string) error {
	// For now, return nil - database operations will be implemented via Supabase REST API
	// This is a placeholder that can be extended with actual Supabase DB client
	fmt.Printf("DEBUG: CreateAccessRequest called for dataset %d\n", datasetID)
	return fmt.Errorf("database operations not yet implemented - use Supabase REST API directly")
}

func (s *SupabaseServiceImpl) GetPendingRequests(ownerAddress string) ([]interface{}, error) {
	fmt.Printf("DEBUG: GetPendingRequests called for owner %s\n", ownerAddress)
	return nil, fmt.Errorf("database operations not yet implemented - use Supabase REST API directly")
}

func (s *SupabaseServiceImpl) ApproveAccessRequest(ownerAddress, requesterAddress string, datasetID uint64) error {
	fmt.Printf("DEBUG: ApproveAccessRequest called for dataset %d\n", datasetID)
	return fmt.Errorf("database operations not yet implemented - use Supabase REST API directly")
}

func (s *SupabaseServiceImpl) DenyAccessRequest(ownerAddress, requesterAddress string, datasetID uint64) error {
	fmt.Printf("DEBUG: DenyAccessRequest called for dataset %d\n", datasetID)
	return fmt.Errorf("database operations not yet implemented - use Supabase REST API directly")
}

func (s *SupabaseServiceImpl) ConfirmPayment(ownerAddress, requesterAddress string, datasetID uint64, txHash string) error {
	fmt.Printf("DEBUG: ConfirmPayment called for dataset %d, tx: %s\n", datasetID, txHash)
	return fmt.Errorf("database operations not yet implemented - use Supabase REST API directly")
}

func (s *SupabaseServiceImpl) GetUserRequests(requesterAddress string) ([]interface{}, error) {
	fmt.Printf("DEBUG: GetUserRequests called for requester %s\n", requesterAddress)
	return nil, fmt.Errorf("database operations not yet implemented - use Supabase REST API directly")
}
