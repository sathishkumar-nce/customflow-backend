package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type OpenAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type Message struct {
	Role    string        `json:"role"`
	Content []ContentItem `json:"content"`
}

type ContentItem struct {
	Type     string    `json:"type"`
	Text     *string   `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type OpenAIResponse struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
	Model   string   `json:"model"`
	Error   *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type Choice struct {
	Message      MessageResponse `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

type MessageResponse struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

var aiService *AIService

type AIService struct {
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   int
	BaseURL     string
}

func InitAIService() {
	aiService = &AIService{
		APIKey:      "",
		Model:       "gpt-4o", // GPT-4o supports vision
		Temperature: 0.7,
		MaxTokens:   1000,
		BaseURL:     "https://api.openai.com/v1/chat/completions",
	}

	if aiService.APIKey == "" {
		log.Println("WARNING: OPENAI_API_KEY not set. AI features will use fallback responses.")
	} else {
		log.Println("AI Service initialized with OpenAI API")
	}
}

// ExtractTextFromImages - Real OCR using OpenAI Vision API
func ExtractTextFromImages(images []string) (string, error) {
	if len(images) == 0 {
		return "", fmt.Errorf("no images provided")
	}

	if aiService.APIKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured")
	}

	log.Printf("Starting OCR for %d images: %v", len(images), images)

	var extractedTexts []string

	for i, imagePath := range images {
		log.Printf("Processing image %d/%d: %s", i+1, len(images), imagePath)

		// Convert image to base64
		base64Image, err := imageToBase64(imagePath)
		if err != nil {
			log.Printf("Failed to convert image %s to base64: %v", imagePath, err)
			continue
		}

		// Create vision request
		extractedText, err := performOCRRequest(base64Image)
		if err != nil {
			log.Printf("OCR failed for image %s: %v", imagePath, err)
			continue
		}

		if strings.TrimSpace(extractedText) != "" {
			extractedTexts = append(extractedTexts, strings.TrimSpace(extractedText))
			log.Printf("Successfully extracted text from %s: %d characters", imagePath, len(extractedText))
		}
	}

	if len(extractedTexts) == 0 {
		return "", fmt.Errorf("could not extract text from any of the %d images", len(images))
	}

	finalText := strings.Join(extractedTexts, "\n\n---NEXT IMAGE---\n\n")
	log.Printf("OCR completed. Total extracted text: %d characters from %d images", len(finalText), len(extractedTexts))

	return finalText, nil
}

// Convert image file to base64
func imageToBase64(imagePath string) (string, error) {
	// Build full path
	fullPath := filepath.Join("./uploads", imagePath)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", fmt.Errorf("image file does not exist: %s", fullPath)
	}

	// Read file
	imageBytes, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read image file %s: %v", fullPath, err)
	}

	// Get file extension to determine MIME type
	ext := strings.ToLower(filepath.Ext(imagePath))
	var mimeType string
	switch ext {
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	default:
		mimeType = "image/jpeg" // Default fallback
	}

	// Convert to base64
	base64String := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64String)

	log.Printf("Converted image %s to base64: %s, size: %d bytes", imagePath, mimeType, len(imageBytes))
	return dataURL, nil
}

// Perform OCR request to OpenAI Vision API
func performOCRRequest(base64Image string) (string, error) {
	messages := []Message{
		{
			Role: "user",
			Content: []ContentItem{
				{
					Type: "text",
					Text: &[]string{"Please extract ALL text from this image. This could be a screenshot of customer messages, order details, specifications, or any other text content. Return only the extracted text content without any additional commentary, formatting, or explanations. If you see table dimensions, customer names, order details, or any specifications, include everything exactly as written."}[0],
				},
				{
					Type: "image_url",
					ImageURL: &ImageURL{
						URL:    base64Image,
						Detail: "high", // Use high detail for better OCR
					},
				},
			},
		},
	}

	requestBody := OpenAIRequest{
		Model:       "gpt-4o",
		Messages:    messages,
		MaxTokens:   500,
		Temperature: 0.1, // Low temperature for accurate extraction
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", aiService.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+aiService.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != 200 {
		log.Printf("OpenAI API error response: %s", string(body))
		return "", fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var openAIResp OpenAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if openAIResp.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s", openAIResp.Error.Message)
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices from OpenAI")
	}

	extractedText := openAIResp.Choices[0].Message.Content

	// Log usage for monitoring
	if openAIResp.Usage.TotalTokens > 0 {
		log.Printf("OCR API Usage - Tokens: %d (Prompt: %d, Completion: %d)",
			openAIResp.Usage.TotalTokens,
			openAIResp.Usage.PromptTokens,
			openAIResp.Usage.CompletionTokens)
	}

	return extractedText, nil
}

// GenerateAIResponse - Generate response using OpenAI
func GenerateAIResponse(message, tone string) (string, error) {
	if aiService.APIKey == "" {
		// Fallback response when no API key is configured
		return generateFallbackResponse(message, tone), nil
	}

	prompt := createPrompt(message, tone)

	requestBody := OpenAIRequest{
		Model:       aiService.Model,
		Temperature: aiService.Temperature,
		MaxTokens:   aiService.MaxTokens,
		Messages: []Message{
			{
				Role: "system",
				Content: []ContentItem{
					{
						Type: "text",
						Text: &[]string{createSystemPrompt()}[0],
					},
				},
			},
			{
				Role: "user",
				Content: []ContentItem{
					{
						Type: "text",
						Text: &prompt,
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", aiService.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+aiService.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var openAIResp OpenAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices from OpenAI")
	}

	return openAIResp.Choices[0].Message.Content, nil
}

func createSystemPrompt() string {
	return `You are a professional customer service assistant for CustomFlow, a premium custom table cover manufacturing business. 

Your role:
- Provide helpful, accurate information about custom table covers
- Maintain a professional yet approachable tone
- Focus on dimensions, materials, delivery timelines, and customization options
- Always prioritize customer satisfaction
- Keep responses concise but informative

Key information about our business:
- We specialize in custom table covers for dining tables, office tables, conference tables
- Materials: Various thicknesses (1mm, 1.5mm, 2mm, 3mm) and corner styles (sharp, rounded, custom)
- Standard delivery: 3-5 business days
- We serve customers through Amazon, WhatsApp, SMS, and phone orders
- Premium quality and precise measurements are our specialties
- We measure in inches

Always be helpful and ensure customers have the information they need to place their order.`
}

func generateFallbackResponse(message, tone string) string {
	responses := map[string][]string{
		"friendly": {
			"Thank you for reaching out about custom table covers! I'd be happy to help you with your order. Could you please share the dimensions and any specific requirements?",
			"Hi there! Thanks for your interest in our table covers. We create high-quality custom covers to fit perfectly. What size do you need?",
		},
		"formal": {
			"Thank you for your inquiry. We would be pleased to assist with your custom table cover requirements. Please provide dimensions and specifications.",
			"We acknowledge your request for custom table cover services. Kindly provide the measurements and material preferences.",
		},
		"short": {
			"Thanks! Please send table dimensions for a quote.",
			"Hi! What size table cover do you need?",
		},
	}

	responseList, exists := responses[tone]
	if !exists {
		responseList = responses["friendly"]
	}

	return responseList[len(message)%len(responseList)]
}

func createPrompt(customerMessage, tone string) string {
	basePrompt := fmt.Sprintf("Customer message: \"%s\"\n\n", customerMessage)

	switch tone {
	case "formal":
		basePrompt += `Generate a formal, professional response for business correspondence. Use proper business language while addressing table cover requirements.`
	case "short":
		basePrompt += `Generate a brief, concise response under 50 words. Focus on essential information - dimensions, material, and delivery.`
	default: // friendly
		basePrompt += `Generate a warm, friendly response while remaining professional. Show enthusiasm for helping with custom table cover needs.`
	}

	return basePrompt
}

// GetModelInfo returns information about the current AI model
func GetModelInfo() map[string]interface{} {
	return map[string]interface{}{
		"model":       aiService.Model,
		"provider":    "OpenAI",
		"temperature": aiService.Temperature,
		"max_tokens":  aiService.MaxTokens,
		"has_api_key": aiService.APIKey != "",
		"vision_ocr":  true,
	}
}

// SetAIParameters allows runtime configuration of AI parameters
func SetAIParameters(temperature float64, maxTokens int) {
	if temperature >= 0 && temperature <= 2 {
		aiService.Temperature = temperature
	}
	if maxTokens > 0 && maxTokens <= 4000 {
		aiService.MaxTokens = maxTokens
	}
}
