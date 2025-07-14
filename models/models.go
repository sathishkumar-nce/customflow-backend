// models/models.go - Compatible with your Flyway schema
package models

import (
	"time"
)

// User model - matches your Flyway migration
type User struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Username  string    `json:"username" gorm:"column:username"`
	Email     string    `json:"email" gorm:"column:email"`
	Password  string    `json:"-" gorm:"column:password"`
	Role      string    `json:"role" gorm:"column:role"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// Order model - matches your Flyway schema exactly
type Order struct {
	ID            uint          `json:"id" gorm:"primaryKey;column:id"`
	OrderID       string        `json:"order_id" gorm:"column:order_id"`
	CustomerName  string        `json:"customer_name" gorm:"column:customer_name"`
	Source        string        `json:"source" gorm:"column:source"`
	PhoneNumber   string        `json:"phone_number" gorm:"column:phone_number"`
	Length        float64       `json:"length" gorm:"column:length;type:decimal(10,2)"`
	Width         float64       `json:"width" gorm:"column:width;type:decimal(10,2)"`
	Thickness     string        `json:"thickness" gorm:"column:thickness"`
	CornerStyle   string        `json:"corner_style" gorm:"column:corner_style"`
	Notes         string        `json:"notes" gorm:"column:notes;type:text"`
	SpecialNotes  string        `json:"special_notes" gorm:"column:special_notes;type:text"`
	Status        string        `json:"status" gorm:"column:status"`
	Images        []OrderImage  `json:"images" gorm:"foreignKey:OrderID"`
	CreatedBy     uint          `json:"created_by" gorm:"column:created_by"`
	CreatedAt     time.Time     `json:"created_at" gorm:"column:created_at"`
	UpdatedAt     time.Time     `json:"updated_at" gorm:"column:updated_at"`
}

// OrderImage model - matches your Flyway schema
type OrderImage struct {
	ID        uint      `json:"id" gorm:"primaryKey;column:id"`
	OrderID   uint      `json:"order_id" gorm:"column:order_id"`
	Filename  string    `json:"filename" gorm:"column:filename"`
	Path      string    `json:"path" gorm:"column:path"`
	Size      int64     `json:"size" gorm:"column:size"`
	MimeType  string    `json:"mime_type" gorm:"column:mime_type"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
}

// AIResponse model - matches your Flyway schema
type AIResponse struct {
	ID           uint      `json:"id" gorm:"primaryKey;column:id"`
	UserID       uint      `json:"user_id" gorm:"column:user_id"`
	InputMessage string    `json:"input_message" gorm:"column:input_message;type:text"`
	Response     string    `json:"response" gorm:"column:response;type:text"`
	Tone         string    `json:"tone" gorm:"column:tone"`
	HasImages    bool      `json:"has_images" gorm:"column:has_images"`
	CreatedAt    time.Time `json:"created_at" gorm:"column:created_at"`
}

// Conversation models for AI memory
type ConversationSession struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	SessionID   string    `json:"session_id" gorm:"column:session_id"`
	UserID      uint      `json:"user_id" gorm:"column:user_id"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`
	ExpiresAt   time.Time `json:"expires_at" gorm:"column:expires_at"`
	Active      bool      `json:"active" gorm:"column:active"`
}

type ConversationMessage struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	SessionID  string    `json:"session_id" gorm:"column:session_id"`
	Role       string    `json:"role" gorm:"column:role"`
	Content    string    `json:"content" gorm:"column:content;type:text"`
	Timestamp  time.Time `json:"timestamp" gorm:"column:timestamp"`
	TokenCount int       `json:"token_count" gorm:"column:token_count"`
}

// Table name methods to ensure GORM uses correct table names
func (User) TableName() string {
	return "users"
}

func (Order) TableName() string {
	return "orders"
}

func (OrderImage) TableName() string {
	return "order_images"
}

func (AIResponse) TableName() string {
	return "ai_responses"
}

func (ConversationSession) TableName() string {
	return "conversation_sessions"
}

func (ConversationMessage) TableName() string {
	return "conversation_messages"
}