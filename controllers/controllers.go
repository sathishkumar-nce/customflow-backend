// =================================================================
// controllers/orders.go - Updated without authentication
package controllers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"customflow/config"
	"customflow/models"

	"github.com/gin-gonic/gin"
	"github.com/twinj/uuid"
	"gorm.io/gorm"
)

type OCRRequest struct {
	Images []string `json:"images" binding:"required"`
}

func UploadFiles(c *gin.Context) {
	// Parse multipart form with 32MB max memory
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large (max 32MB)"})
		return
	}

	files := c.Request.MultipartForm.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files uploaded"})
		return
	}

	// Create uploads directory if it doesn't exist
	uploadsDir := "./uploads"
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create uploads directory"})
		return
	}

	var uploadedFiles []gin.H
	var failedFiles []string

	for _, fileHeader := range files {
		// Validate file type
		if !isValidImageType(fileHeader.Filename) {
			failedFiles = append(failedFiles, fileHeader.Filename+" (invalid type)")
			continue
		}

		// Validate file size (max 10MB per file)
		if fileHeader.Size > 10<<20 {
			failedFiles = append(failedFiles, fileHeader.Filename+" (too large)")
			continue
		}

		// Generate unique filename
		ext := filepath.Ext(fileHeader.Filename)
		filename := fmt.Sprintf("%s_%d%s",
			strings.ReplaceAll(uuid.New([]byte{001}).String(), "-", ""),
			time.Now().Unix(),
			ext)

		filePath := filepath.Join(uploadsDir, filename)

		// Save file
		if err := c.SaveUploadedFile(fileHeader, filePath); err != nil {
			failedFiles = append(failedFiles, fileHeader.Filename+" (save failed)")
			continue
		}

		uploadedFiles = append(uploadedFiles, gin.H{
			"filename":      filename,
			"original_name": fileHeader.Filename,
			"size":          fileHeader.Size,
			"url":           fmt.Sprintf("/uploads/%s", filename),
			"mime_type":     getMimeType(ext),
		})
	}

	// Response
	response := gin.H{
		"message":        fmt.Sprintf("Processed %d files", len(files)),
		"uploaded_count": len(uploadedFiles),
		"failed_count":   len(failedFiles),
		"files":          uploadedFiles,
	}

	if len(failedFiles) > 0 {
		response["failed_files"] = failedFiles
	}

	if len(uploadedFiles) == 0 {
		c.JSON(http.StatusBadRequest, response)
	} else {
		c.JSON(http.StatusOK, response)
	}
}

// Helper function to validate image file types
func isValidImageType(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	validExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg"}

	for _, validExt := range validExts {
		if ext == validExt {
			return true
		}
	}
	return false
}

func HealthCheck(c *gin.Context) {
	// Check database connection
	sqlDB, err := config.DB.DB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":    "unhealthy",
			"timestamp": time.Now(),
			"error":     "Database connection failed",
		})
		return
	}

	if err := sqlDB.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":    "unhealthy",
			"timestamp": time.Now(),
			"error":     "Database ping failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "healthy",
		"timestamp":   time.Now(),
		"database":    "connected",
		"ai_service":  "available",
		"version":     "1.0.0",
		"environment": "no-auth",
		"upload_dir":  "./uploads",
	})
}

type CreateOrderRequest struct {
	OrderID      string   `json:"order_id" binding:"required,min=3,max=100"`
	CustomerName string   `json:"customer_name"`
	Source       string   `json:"source"`
	PhoneNumber  string   `json:"phone_number"`
	Length       float64  `json:"length" binding:"required,gt=0"`
	Width        float64  `json:"width" binding:"required,gt=0"`
	Thickness    string   `json:"thickness" binding:"required"`
	CornerStyle  string   `json:"corner_style" binding:"required"`
	Notes        string   `json:"notes"`
	SpecialNotes string   `json:"special_notes"`
	ImageFiles   []string `json:"image_files"`
}

type UpdateOrderStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// GetOrders - Fixed for Flyway schema
func GetOrders(c *gin.Context) {
	log.Println("GetOrders: Starting request")

	var orders []models.Order

	// Validate database connection
	if config.DB == nil {
		log.Println("GetOrders: Database connection is nil")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection error"})
		return
	}

	// Test database connectivity
	sqlDB, err := config.DB.DB()
	if err != nil {
		log.Printf("GetOrders: Failed to get underlying sql.DB: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection error"})
		return
	}

	if err := sqlDB.Ping(); err != nil {
		log.Printf("GetOrders: Database ping failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not responding"})
		return
	}

	// Build query with proper preloading
	query := config.DB.Table("orders").Preload("Images")

	// Apply filters
	status := strings.TrimSpace(c.Query("status"))
	if status != "" {
		// Validate status against your Flyway schema constraints
		validStatuses := []string{"new", "in-progress", "done"} // Based on your schema
		if !contains(validStatuses, status) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status filter"})
			return
		}
		query = query.Where("status = ?", status)
	}

	search := strings.TrimSpace(c.Query("search"))
	if search != "" {
		// Escape for SQL injection prevention
		search = strings.ReplaceAll(search, "'", "''")
		query = query.Where("order_id ILIKE ? OR customer_name ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Pagination
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	// Get total count
	var total int64
	countQuery := config.DB.Table("orders")
	if status != "" {
		countQuery = countQuery.Where("status = ?", status)
	}
	if search != "" {
		countQuery = countQuery.Where("order_id ILIKE ? OR customer_name ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if err := countQuery.Count(&total).Error; err != nil {
		log.Printf("GetOrders: Failed to count orders: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count orders"})
		return
	}

	log.Printf("GetOrders: Found %d total orders", total)

	// Get orders with proper joins for images
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&orders).Error; err != nil {
		log.Printf("GetOrders: Failed to fetch orders: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders: " + err.Error()})
		return
	}

	// Manually load images for each order if preload didn't work
	for i := range orders {
		config.DB.Where("order_id = ?", orders[i].ID).Find(&orders[i].Images)
	}

	log.Printf("GetOrders: Successfully fetched %d orders", len(orders))

	c.JSON(http.StatusOK, gin.H{
		"orders": orders,
		"pagination": gin.H{
			"total":    total,
			"page":     page,
			"limit":    limit,
			"pages":    (total + int64(limit) - 1) / int64(limit),
			"has_next": page < int((total+int64(limit)-1)/int64(limit)),
			"has_prev": page > 1,
		},
	})
}

// GetOrder - Fixed for Flyway schema
func GetOrder(c *gin.Context) {
	id := c.Param("id")
	log.Printf("GetOrder: Fetching order with ID: %s", id)

	orderID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID format"})
		return
	}

	var order models.Order
	if err := config.DB.Where("id = ?", orderID).First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		} else {
			log.Printf("GetOrder: Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	// Load images separately
	config.DB.Where("order_id = ?", order.ID).Find(&order.Images)

	log.Printf("GetOrder: Successfully found order: %s", order.OrderID)
	c.JSON(http.StatusOK, gin.H{"order": order})
}

// CreateOrder - Fixed for Flyway schema
func CreateOrder(c *gin.Context) {
	log.Println("CreateOrder: Starting order creation")

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("CreateOrder: Validation error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed: " + err.Error()})
		return
	}

	// Set defaults based on your Flyway schema
	if req.Source == "" {
		req.Source = "amazon"
	}
	if req.Thickness == "" {
		req.Thickness = "3mm"
	}
	if req.CornerStyle == "" {
		req.CornerStyle = "sharp"
	}

	// Validate against your Flyway schema constraints
	validSources := []string{"amazon", "whatsapp", "sms", "call"}
	if !contains(validSources, req.Source) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid source"})
		return
	}

	validThickness := []string{"2mm", "3mm", "5mm", "8mm"} // Based on your migration
	if !contains(validThickness, req.Thickness) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid thickness"})
		return
	}

	validCorners := []string{"sharp", "rounded", "custom"}
	if !contains(validCorners, req.CornerStyle) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid corner style"})
		return
	}

	// Normalize order ID
	req.OrderID = strings.TrimSpace(req.OrderID)
	if req.OrderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order ID cannot be empty"})
		return
	}

	// Check for duplicate order ID
	var existingOrder models.Order
	result := config.DB.Where("order_id = ?", req.OrderID).First(&existingOrder)
	if result.Error == nil {
		log.Printf("CreateOrder: Duplicate order ID found: %s", req.OrderID)
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("Order ID '%s' already exists", req.OrderID),
			"existing_order": gin.H{
				"id":         existingOrder.ID,
				"order_id":   existingOrder.OrderID,
				"created_at": existingOrder.CreatedAt.Format("2006-01-02 15:04:05"),
				"status":     existingOrder.Status,
			},
		})
		return
	} else if result.Error != gorm.ErrRecordNotFound {
		log.Printf("CreateOrder: Database error checking duplicate: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error checking order ID"})
		return
	}

	// Validate image files
	var validImageFiles []string
	for _, imageFile := range req.ImageFiles {
		imagePath := filepath.Join("./uploads", imageFile)
		if _, err := os.Stat(imagePath); err == nil {
			validImageFiles = append(validImageFiles, imageFile)
			log.Printf("CreateOrder: Valid image file: %s", imageFile)
		} else {
			log.Printf("CreateOrder: Image file not found: %s", imagePath)
		}
	}

	// Create order matching your Flyway schema exactly
	order := models.Order{
		OrderID:      req.OrderID,
		CustomerName: strings.TrimSpace(req.CustomerName),
		Source:       req.Source,
		PhoneNumber:  strings.TrimSpace(req.PhoneNumber),
		Length:       req.Length,
		Width:        req.Width,
		Thickness:    req.Thickness,
		CornerStyle:  req.CornerStyle,
		Notes:        strings.TrimSpace(req.Notes),
		SpecialNotes: strings.TrimSpace(req.SpecialNotes),
		Status:       "new", // Default status based on your schema
		CreatedBy:    1,     // Default user
	}

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		log.Printf("CreateOrder: Failed to start transaction: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction error"})
		return
	}

	// Create order
	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		log.Printf("CreateOrder: Failed to create order: %v", err)

		// Check if it's a duplicate key error
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
			strings.Contains(strings.ToLower(err.Error()), "unique") {
			c.JSON(http.StatusConflict, gin.H{"error": "Order ID already exists"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order: " + err.Error()})
		}
		return
	}

	// Add images if any valid ones exist
	for _, filename := range validImageFiles {
		image := models.OrderImage{
			OrderID:  order.ID,
			Filename: filename,
			Path:     fmt.Sprintf("/uploads/%s", filename),
			MimeType: getMimeType(filepath.Ext(filename)),
		}

		// Get file size
		if stat, err := os.Stat(filepath.Join("./uploads", filename)); err == nil {
			image.Size = stat.Size()
		}

		if err := tx.Create(&image).Error; err != nil {
			log.Printf("CreateOrder: Failed to create image record for %s: %v", filename, err)
			// Continue with order creation even if image fails
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("CreateOrder: Failed to commit transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save order"})
		return
	}

	// Reload order with images
	config.DB.Where("order_id = ?", order.OrderID).First(&order)
	config.DB.Where("order_id = ?", order.ID).Find(&order.Images)

	log.Printf("CreateOrder: Successfully created order: %s (ID: %d)", order.OrderID, order.ID)
	c.JSON(http.StatusCreated, gin.H{
		"order":   order,
		"message": "Order created successfully",
	})
}

// UpdateOrder - Fixed for Flyway schema
func UpdateOrder(c *gin.Context) {
	id := c.Param("id")
	log.Printf("UpdateOrder: Updating order ID: %s", id)

	orderID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID format"})
		return
	}

	var order models.Order
	if err := config.DB.Where("id = ?", orderID).First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed: " + err.Error()})
		return
	}

	// Check for duplicate order ID if changed
	if req.OrderID != order.OrderID {
		var existingOrder models.Order
		result := config.DB.Where("order_id = ? AND id != ?", req.OrderID, order.ID).First(&existingOrder)
		if result.Error == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Order ID already exists: " + req.OrderID})
			return
		}
	}

	// Start transaction
	tx := config.DB.Begin()

	// Update order fields
	order.OrderID = strings.TrimSpace(req.OrderID)
	order.CustomerName = strings.TrimSpace(req.CustomerName)
	order.Source = req.Source
	order.PhoneNumber = strings.TrimSpace(req.PhoneNumber)
	order.Length = req.Length
	order.Width = req.Width
	order.Thickness = req.Thickness
	order.CornerStyle = req.CornerStyle
	order.Notes = strings.TrimSpace(req.Notes)
	order.SpecialNotes = strings.TrimSpace(req.SpecialNotes)

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		log.Printf("UpdateOrder: Failed to update order: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order"})
		return
	}

	// Update images if provided
	if len(req.ImageFiles) > 0 {
		// Delete existing images
		tx.Where("order_id = ?", order.ID).Delete(&models.OrderImage{})

		// Add new images
		for _, filename := range req.ImageFiles {
			if _, err := os.Stat(filepath.Join("./uploads", filename)); err == nil {
				image := models.OrderImage{
					OrderID:  order.ID,
					Filename: filename,
					Path:     fmt.Sprintf("/uploads/%s", filename),
					MimeType: getMimeType(filepath.Ext(filename)),
				}

				if stat, err := os.Stat(filepath.Join("./uploads", filename)); err == nil {
					image.Size = stat.Size()
				}

				tx.Create(&image)
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save changes"})
		return
	}

	// Reload with images
	config.DB.Where("order_id = ?", order.ID).Find(&order.Images)

	log.Printf("UpdateOrder: Successfully updated order: %s", order.OrderID)
	c.JSON(http.StatusOK, gin.H{"order": order})
}

// UpdateOrderStatus - Fixed for Flyway schema
func UpdateOrderStatus(c *gin.Context) {
	id := c.Param("id")
	log.Printf("UpdateOrderStatus: Updating status for order ID: %s", id)

	orderID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID format"})
		return
	}

	var req UpdateOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate status against Flyway schema
	validStatuses := []string{"new", "in-progress", "done"}
	if !contains(validStatuses, req.Status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
		return
	}

	var order models.Order
	if err := config.DB.Where("id = ?", orderID).First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	oldStatus := order.Status
	order.Status = req.Status

	if err := config.DB.Save(&order).Error; err != nil {
		log.Printf("UpdateOrderStatus: Failed to update status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}

	log.Printf("UpdateOrderStatus: Status updated from %s to %s for order %s", oldStatus, req.Status, order.OrderID)
	c.JSON(http.StatusOK, gin.H{"order": order})
}

// DeleteOrder - Fixed for Flyway schema
func DeleteOrder(c *gin.Context) {
	id := c.Param("id")
	log.Printf("DeleteOrder: Deleting order ID: %s", id)

	orderID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID format"})
		return
	}

	var order models.Order
	if err := config.DB.Where("id = ?", orderID).First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	// Start transaction
	tx := config.DB.Begin()

	// Delete images first (foreign key constraint)
	if err := tx.Where("order_id = ?", order.ID).Delete(&models.OrderImage{}).Error; err != nil {
		tx.Rollback()
		log.Printf("DeleteOrder: Failed to delete images: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete order images"})
		return
	}

	// Delete order
	if err := tx.Delete(&order).Error; err != nil {
		tx.Rollback()
		log.Printf("DeleteOrder: Failed to delete order: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete order"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete deletion"})
		return
	}

	log.Printf("DeleteOrder: Successfully deleted order: %s", order.OrderID)
	c.JSON(http.StatusOK, gin.H{"message": "Order deleted successfully"})
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func getMimeType(ext string) string {
	mimeTypes := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".bmp":  "image/bmp",
		".svg":  "image/svg+xml",
	}

	if mimeType, exists := mimeTypes[strings.ToLower(ext)]; exists {
		return mimeType
	}
	return "application/octet-stream"
}
