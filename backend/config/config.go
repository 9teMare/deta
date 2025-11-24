package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	AptosNodeURL       string
	AptosIndexerURL    string // Aptos Indexer API URL
	AptosIndexerAPIKey string // Aptos Indexer API Key
	UseIndexer         bool   // Toggle to enable/disable indexer usage
	DataXModuleAddr    string
	NetworkModuleAddr  string
	ChainID            uint8
	SupabaseS3URL      string
	SupabaseKey        string
	SupabaseBucket     string
	SupabaseAccessKey  string // S3 access key (if using S3 SDK)
	SupabaseSecretKey  string // S3 secret key (if using S3 SDK)
	ShelbyRPCURL       string
	ShelbyAccountKey   string
}

var AppConfig *Config

func LoadConfig() error {
	// Load .env file if it exists
	_ = godotenv.Load()

	AppConfig = &Config{
		Port:               getEnv("PORT", "8080"),
		AptosNodeURL:       getEnv("APTOS_NODE_URL", "https://fullnode.testnet.aptoslabs.com"),
		AptosIndexerURL:    getEnv("APTOS_INDEXER_URL", "https://api.testnet.aptoslabs.com/v1/graphql"),
		AptosIndexerAPIKey: getEnv("APTOS_INDEXER_API_KEY", "aptoslabs_gFwzfgw2qNK_PoVDshwNdcPq8gKAn9MMwjc3nydopPU5k"),
		UseIndexer:         getEnvAsBool("USE_INDEXER", "true"), // Enable indexer by default
		DataXModuleAddr:    getEnv("DATAX_MODULE_ADDR", "0x0b133cba97a77b2dee290919e27c72c7d49d8bf5a3294efbd8c40cc38a009eab"),
		NetworkModuleAddr:  getEnv("NETWORK_MODULE_ADDR", "0x0b133cba97a77b2dee290919e27c72c7d49d8bf5a3294efbd8c40cc38a009eab"),
		ChainID:            uint8(getEnvAsInt("CHAIN_ID", "2")), // 2 for testnet
		SupabaseS3URL:      getEnv("SUPABASE_S3_URL", ""),
		SupabaseKey:        getEnv("SUPABASE_KEY", ""),
		SupabaseBucket:     getEnv("SUPABASE_BUCKET", "csv-data"), // Supabase storage bucket name
		SupabaseAccessKey:  getEnv("SUPABASE_ACCESS_KEY", ""),     // S3 access key (if using S3 SDK)
		SupabaseSecretKey:  getEnv("SUPABASE_SECRET_KEY", ""),     // S3 secret key (if using S3 SDK)
		ShelbyRPCURL:       getEnv("SHELBY_RPC_URL", ""),
		ShelbyAccountKey:   getEnv("SHELBY_ACCOUNT_KEY", ""),
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue string) int {
	value := os.Getenv(key)
	if value == "" {
		value = defaultValue
	}
	// Convert string to int
	result, err := strconv.Atoi(value)
	if err != nil {
		return 2 // default to testnet
	}
	return result
}

func getEnvAsBool(key string, defaultValue string) bool {
	value := os.Getenv(key)
	if value == "" {
		value = defaultValue
	}
	// Convert string to bool (accepts: true, false, 1, 0, yes, no)
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "true" || value == "1" || value == "yes" {
		return true
	}
	return false
}
