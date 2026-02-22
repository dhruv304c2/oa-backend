package config

import (
	"os"
)

// GetGeminiModel returns the Gemini model to use from environment variable
// Defaults to "gemini-2.5-flash" if not set
func GetGeminiModel() string {
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		// Default to flash model if not specified
		return "gemini-2.5-flash"
	}
	return model
}

// GetGeminiAPIKey returns the Gemini API key from environment variable
func GetGeminiAPIKey() string {
	return os.Getenv("GEMINI_API_KEY")
}

// GetMongoDBURI returns the MongoDB connection URI from environment variable
func GetMongoDBURI() string {
	return os.Getenv("MONGODB_URI")
}

// GetAllowedOrigins returns the allowed CORS origins from environment variable
func GetAllowedOrigins() string {
	return os.Getenv("ALLOWED_ORIGINS")
}