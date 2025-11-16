package main

import (
	"fmt"
	"log"

	"github.com/datax/backend/config"
	"github.com/datax/backend/handlers"
	"github.com/datax/backend/services"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize Aptos service (returns AptosServiceImpl which implements AptosService interface)
	aptosService, err := services.NewAptosService()
	if err != nil {
		log.Fatalf("Failed to initialize Aptos service: %v", err)
	}

	// Initialize Supabase storage service
	storageService := services.NewSupabaseService()

	// Initialize handlers
	handler := handlers.NewHandler(aptosService, storageService)

	// Setup Gin router
	router := gin.Default()

	// CORS middleware
	router.Use(corsMiddleware())

	// Health check
	router.GET("/health", handler.HealthCheck)

	// API routes
	api := router.Group("/api/v1")
	{
		// User initialization
		api.POST("/users/initialize", handler.InitializeUser)
		api.POST("/users/check-initialization", handler.CheckInitialization)

		// Data operations
		api.POST("/data/submit", handler.SubmitData)
		api.POST("/data/delete", handler.DeleteDataset)
		api.POST("/data/get", handler.GetDataset)

		// Access control
		api.POST("/access/grant", handler.GrantAccess)
		api.POST("/access/revoke", handler.RevokeAccess)
		api.POST("/access/check", handler.CheckAccess)

		// Vault operations
		api.POST("/vault/get", handler.GetUserVault)

		// Token operations
		api.POST("/token/register", handler.RegisterToken)
		api.POST("/token/mint", handler.MintToken)

		// CSV upload
		api.POST("/data/submit-csv", handler.SubmitCSV)

		// Marketplace
		api.GET("/marketplace/datasets", handler.GetMarketplaceDatasets)
		api.POST("/marketplace/access-requests", handler.GetAccessRequests)
		api.POST("/marketplace/request-access", handler.RequestAccess)
		api.POST("/marketplace/register-user", handler.RegisterUserForMarketplace)

		// CSV data viewing
		api.POST("/data/get-csv", handler.GetCSVData)
	}

	// Start server
	addr := fmt.Sprintf(":%s", config.AppConfig.Port)
	log.Printf("Server starting on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
