package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port           string
	DatabaseURL    string
	OpenAIAPIKey   string
	OpenAIModel    string
	LLMMockMode    bool
	CORSOrigin     string
}

func Load() Config {
	mock, _ := strconv.ParseBool(os.Getenv("LLM_MOCK_MODE"))
	return Config{
		Port:         getEnv("PORT", "8080"),
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://insights:insights@localhost:5432/insights?sslmode=disable"),
		OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:  getEnv("OPENAI_MODEL", "gpt-4o-mini"),
		LLMMockMode:  mock || os.Getenv("OPENAI_API_KEY") == "",
		CORSOrigin:   getEnv("CORS_ORIGIN", "http://localhost:5173"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
