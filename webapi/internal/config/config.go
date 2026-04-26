package config

import (
	"flag"
	"os"
)

type Config struct {
	ListenAddress     string
	ControllerAddress string
	DatabasePath      string
	AdminUser         string
	AdminPassword     string
	LLMBaseURL        string
	LLMAPIKey         string
	LLMModel          string
}

func Load(args []string) Config {
	cfg := Config{
		ListenAddress:     envOrDefault("MX_API_LISTEN", ":8090"),
		ControllerAddress: envOrDefault("MX_CONTROLLER_ADDR", "127.0.0.1:19081"),
		DatabasePath:      envOrDefault("MX_API_DB", "metalx-webapi.sqlite"),
		AdminUser:         envOrDefault("MX_ADMIN_USER", "admin"),
		AdminPassword:     envOrDefault("MX_ADMIN_PASSWORD", "metalx-admin-2026"),
		LLMBaseURL:        envOrDefault("MX_LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMAPIKey:         os.Getenv("MX_LLM_API_KEY"),
		LLMModel:          envOrDefault("MX_LLM_MODEL", "gpt-4o-mini"),
	}

	fs := flag.NewFlagSet("mxapi", flag.ContinueOnError)
	fs.StringVar(&cfg.ListenAddress, "listen", cfg.ListenAddress, "webapi listen address")
	fs.StringVar(&cfg.ControllerAddress, "controller", cfg.ControllerAddress, "controller base url")
	fs.StringVar(&cfg.DatabasePath, "db", cfg.DatabasePath, "sqlite database path")
	fs.StringVar(&cfg.AdminUser, "user", cfg.AdminUser, "bootstrap admin username")
	fs.StringVar(&cfg.AdminPassword, "password", cfg.AdminPassword, "bootstrap admin password")
	fs.StringVar(&cfg.LLMBaseURL, "llm-base-url", cfg.LLMBaseURL, "OpenAI-compatible API base URL")
	fs.StringVar(&cfg.LLMAPIKey, "llm-api-key", cfg.LLMAPIKey, "OpenAI-compatible API key")
	fs.StringVar(&cfg.LLMModel, "llm-model", cfg.LLMModel, "OpenAI-compatible chat model")
	_ = fs.Parse(args)
	return cfg
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
