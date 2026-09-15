package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Plan string

const (
	PlanFree      Plan = "free"
	PlanPro       Plan = "pro"
	PlanHigherPro Plan = "higher_pro"
)

type Config struct {
	Port string

	// AI Provider Credentials
	GroqAPIKey   string
	GroqBaseURL  string
	AgnesAPIKey  string
	AgnesBaseURL string

	// Reserved for later
	OpenRouterAPIKey string
	DeepSeekAPIKey   string

	DevMode bool
}

func LoadConfig() (*Config, error) {
	cfg := &Config{}

	generalEnv, _ := godotenv.Read(".env")
	cfg.Port = getValue(generalEnv, "PORT", "8080")
	cfg.DevMode = getValueAsBool(generalEnv, "DEV_MODE", false)

	env1, _ := godotenv.Read(".env1")

	// ── Groq (primary) ──
	cfg.GroqAPIKey = getValue(env1, "GROQ_API_KEY", os.Getenv("GROQ_API_KEY"))
	cfg.GroqBaseURL = getValue(env1, "GROQ_BASE_URL", "https://api.groq.com/openai/v1")
	if cfg.GroqAPIKey == "" {
		return nil, &ConfigError{Field: "GROQ_API_KEY", Msg: "Groq API key is required"}
	}

	// ── Agnes (fallback) ──
	cfg.AgnesAPIKey = getValue(env1, "AGNES_API_KEY", os.Getenv("AGNES_API_KEY"))
	cfg.AgnesBaseURL = getValue(env1, "AGNES_BASE_URL", "https://apihub.agnes-ai.com/v1")
	if cfg.AgnesAPIKey == "" {
		log.Println("ℹ️  AGNES_API_KEY not set — no fallback available")
	}

	// ── Reserved for later ──
	cfg.OpenRouterAPIKey = getValue(env1, "OPENROUTER_API_KEY", "")
	env2, _ := godotenv.Read(".env2")
	cfg.DeepSeekAPIKey = getValue(env2, "DEEPSEEK_API_KEY", "")

	log.Println("✅ Configuration loaded: Groq primary + Agnes fallback")
	return cfg, nil
}

func getValue(envMap map[string]string, key, defaultValue string) string {
	if value, exists := envMap[key]; exists && value != "" {
		return value
	}
	return defaultValue
}

func getValueAsBool(envMap map[string]string, key string, defaultValue bool) bool {
	if value, exists := envMap[key]; exists && value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

type ConfigError struct {
	Field string
	Msg   string
}

func (e *ConfigError) Error() string {
	return e.Field + ": " + e.Msg
}
