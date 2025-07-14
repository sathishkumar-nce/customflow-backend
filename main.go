// main.go - Flyway compatible (no schema modifications)
package main

import (
	"fmt"
	"log"
	"os"

	"customflow/config"
	"customflow/controllers"
	"customflow/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize database connection
	log.Println("Connecting to database...")
	config.ConnectDatabase()

	// Verify database connectivity (no migrations)
	if err := verifyDatabaseConnection(); err != nil {
		log.Fatal("Database connection failed:", err)
	}

	// Verify required tables exist (created by Flyway)
	if err := verifyRequiredTables(); err != nil {
		log.Fatal("Required database tables not found. Please run Flyway migrations:", err)
	}

	// Initialize services
	log.Println("Initializing AI service...")
	services.InitAIService()

	// Setup Gin router
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()

	// CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Session-ID"},
		ExposeHeaders:    []string{"Content-Length", "X-Session-ID"},
		AllowCredentials: true,
		MaxAge:           12 * 3600,
	}))

	// Request logging middleware
	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	}))

	// Recovery middleware
	router.Use(gin.Recovery())

	// Static files for uploads
	router.Static("/uploads", "./uploads")

	// Ensure uploads directory exists
	if err := os.MkdirAll("./uploads", 0755); err != nil {
		log.Printf("Warning: Could not create uploads directory: %v", err)
	}

	// API routes
	api := router.Group("/api/v1")
	{
		// Health check
		api.GET("/health", controllers.HealthCheck)

		// Order routes
		orders := api.Group("/orders")
		{
			orders.GET("", controllers.GetOrders)
			orders.GET("/:id", controllers.GetOrder)
			orders.POST("", controllers.CreateOrder)
			orders.PUT("/:id", controllers.UpdateOrder)
			orders.DELETE("/:id", controllers.DeleteOrder)
			orders.PUT("/:id/status", controllers.UpdateOrderStatus)
		}

		// File upload
		api.POST("/upload", controllers.UploadFiles)
	}

	// 404 handler
	router.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"error": "Route not found"})
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "7070"
	}

	log.Printf("🚀 Server starting on port %s", port)
	log.Printf("📊 Database: Connected and verified")
	log.Printf("🤖 AI Service: Initialized")
	log.Printf("📁 Static files: ./uploads")
	log.Printf("🌐 API endpoints: /api/v1")
	log.Printf("⚙️  Schema management: Flyway")

	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

// Verify database connection without modifying schema
func verifyDatabaseConnection() error {
	sqlDB, err := config.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %v", err)
	}

	log.Println("✓ Database connection verified")
	return nil
}

// Verify that Flyway has created required tables
func verifyRequiredTables() error {
	requiredTables := []string{
		"users",
		"orders",
		"order_images",
		"ai_responses",
	}

	for _, tableName := range requiredTables {
		var exists bool
		query := "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = ?)"

		if err := config.DB.Raw(query, tableName).Scan(&exists).Error; err != nil {
			return fmt.Errorf("failed to check table %s: %v", tableName, err)
		}

		if !exists {
			return fmt.Errorf("table '%s' does not exist - please run Flyway migrations", tableName)
		}

		log.Printf("✓ Table '%s' exists", tableName)
	}

	log.Println("✓ All required tables verified")
	return nil
}

// Optional: Database diagnostic function
func runDatabaseDiagnostics() {
	log.Println("\n=== DATABASE DIAGNOSTICS ===")

	// Count records in each table
	tables := map[string]string{
		"users":        "SELECT COUNT(*) FROM users",
		"orders":       "SELECT COUNT(*) FROM orders",
		"order_images": "SELECT COUNT(*) FROM order_images",
		"ai_responses": "SELECT COUNT(*) FROM ai_responses",
	}

	for tableName, query := range tables {
		var count int64
		if err := config.DB.Raw(query).Scan(&count).Error; err != nil {
			log.Printf("✗ Failed to count %s: %v", tableName, err)
		} else {
			log.Printf("📊 %s: %d records", tableName, count)
		}
	}

	// Check for any orders with duplicate order_id
	var duplicates []struct {
		OrderID string `json:"order_id"`
		Count   int64  `json:"count"`
	}

	err := config.DB.Raw("SELECT order_id, COUNT(*) as count FROM orders GROUP BY order_id HAVING COUNT(*) > 1").Scan(&duplicates).Error
	if err != nil {
		log.Printf("✗ Failed to check duplicates: %v", err)
	} else if len(duplicates) > 0 {
		log.Printf("⚠️  Found %d duplicate order IDs:", len(duplicates))
		for _, dup := range duplicates {
			log.Printf("   - OrderID '%s' appears %d times", dup.OrderID, dup.Count)
		}
	} else {
		log.Println("✓ No duplicate order IDs found")
	}

	log.Println("=== DIAGNOSTICS COMPLETE ===\n")
}
