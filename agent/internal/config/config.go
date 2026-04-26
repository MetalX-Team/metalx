package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type Config struct {
	NodeID            string
	NodeName          string
	ControllerAddress string
	DiscoveryUDPPort  int
	ListenAddress     string
	GRPCListenAddress string
	ReportInterval    time.Duration
}

func Load(args []string) (Config, error) {
	cfg := Config{
		NodeID:            envOrDefault("MX_AGENT_ID", "node-"+hostOrDefault("localhost")),
		NodeName:          envOrDefault("MX_AGENT_NAME", hostOrDefault("localhost")),
		ControllerAddress: envOrDefault("MX_CONTROLLER_ADDR", "127.0.0.1:19081"),
		DiscoveryUDPPort:  9527,
		ListenAddress:     envOrDefault("MX_AGENT_LISTEN", ":18081"),
		GRPCListenAddress: envOrDefault("MX_AGENT_GRPC_LISTEN", ":19091"),
		ReportInterval:    1 * time.Second,
	}

	fs := flag.NewFlagSet("mxagent", flag.ContinueOnError)
	fs.StringVar(&cfg.NodeID, "id", cfg.NodeID, "unique agent id")
	fs.StringVar(&cfg.NodeName, "name", cfg.NodeName, "node display name")
	fs.StringVar(&cfg.ControllerAddress, "controller", cfg.ControllerAddress, "controller address")
	fs.IntVar(&cfg.DiscoveryUDPPort, "discovery-port", cfg.DiscoveryUDPPort, "controller discovery port")
	fs.StringVar(&cfg.ListenAddress, "listen", cfg.ListenAddress, "agent http listen address")
	fs.StringVar(&cfg.GRPCListenAddress, "grpc-listen", cfg.GRPCListenAddress, "agent grpc listen address")
	fs.DurationVar(&cfg.ReportInterval, "interval", cfg.ReportInterval, "metric report interval")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func hostOrDefault(fallback string) string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return fallback
	}
	return host
}

func (c Config) Validate() error {
	if c.ControllerAddress == "" {
		return fmt.Errorf("controller address is required")
	}
	return nil
}
