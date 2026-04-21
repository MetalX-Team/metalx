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
}

func Load(args []string) Config {
	cfg := Config{
		ListenAddress:     envOrDefault("MX_API_LISTEN", ":8090"),
		ControllerAddress: envOrDefault("MX_CONTROLLER_ADDR", "127.0.0.1:19081"),
		DatabasePath:      envOrDefault("MX_API_DB", "metalx-webapi.sqlite"),
		AdminUser:         envOrDefault("MX_ADMIN_USER", "admin"),
		AdminPassword:     envOrDefault("MX_ADMIN_PASSWORD", "metalx-admin-2026"),
	}

	fs := flag.NewFlagSet("mxapi", flag.ContinueOnError)
	fs.StringVar(&cfg.ListenAddress, "listen", cfg.ListenAddress, "webapi listen address")
	fs.StringVar(&cfg.ControllerAddress, "controller", cfg.ControllerAddress, "controller base url")
	fs.StringVar(&cfg.DatabasePath, "db", cfg.DatabasePath, "sqlite database path")
	fs.StringVar(&cfg.AdminUser, "user", cfg.AdminUser, "bootstrap admin username")
	fs.StringVar(&cfg.AdminPassword, "password", cfg.AdminPassword, "bootstrap admin password")
	_ = fs.Parse(args)
	return cfg
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
