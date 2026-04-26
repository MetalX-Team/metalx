package config

import (
	"flag"
	"os"
)

type Config struct {
	ListenAddress       string
	GRPCListenAddress   string
	DiscoveryPort       int
	DatabasePath        string
	DNSMasqStateDir     string
	ProvisioningBaseURL string
	PublicGRPCAddress   string
	AgentBinaryPath     string
	AuthToken           string
	AllowedShell        bool
	DefaultNodeAddr     string
}

func Load(args []string) Config {
	cfg := Config{
		ListenAddress:       envOrDefault("MX_CONTROLLER_LISTEN", ":8081"),
		GRPCListenAddress:   envOrDefault("MX_CONTROLLER_GRPC_LISTEN", ":19081"),
		DiscoveryPort:       9527,
		DatabasePath:        envOrDefault("MX_CONTROLLER_DB", "metalx-controller.sqlite"),
		DNSMasqStateDir:     envOrDefault("MX_DNSMASQ_STATE_DIR", "runtime/dnsmasq"),
		ProvisioningBaseURL: envOrDefault("MX_PROVISIONING_BASE_URL", "http://127.0.0.1:8081"),
		PublicGRPCAddress:   envOrDefault("MX_PUBLIC_CONTROLLER_GRPC_ADDR", "127.0.0.1:19081"),
		AgentBinaryPath:     envOrDefault("MX_AGENT_BINARY_PATH", "bin/mxagent"),
		AuthToken:           envOrDefault("MX_CONTROLLER_TOKEN", "dev-controller-token"),
		AllowedShell:        true,
		DefaultNodeAddr:     envOrDefault("MX_AGENT_FALLBACK_ADDR", "127.0.0.1:19091"),
	}

	fs := flag.NewFlagSet("mxctl", flag.ContinueOnError)
	fs.StringVar(&cfg.ListenAddress, "listen", cfg.ListenAddress, "controller listen address")
	fs.StringVar(&cfg.GRPCListenAddress, "grpc-listen", cfg.GRPCListenAddress, "controller grpc listen address")
	fs.IntVar(&cfg.DiscoveryPort, "discovery-port", cfg.DiscoveryPort, "udp discovery port")
	fs.StringVar(&cfg.DatabasePath, "db", cfg.DatabasePath, "sqlite database path")
	fs.StringVar(&cfg.DNSMasqStateDir, "dnsmasq-dir", cfg.DNSMasqStateDir, "dnsmasq generated state directory")
	fs.StringVar(&cfg.ProvisioningBaseURL, "provisioning-base", cfg.ProvisioningBaseURL, "public provisioning base URL")
	fs.StringVar(&cfg.PublicGRPCAddress, "public-grpc-addr", cfg.PublicGRPCAddress, "public controller gRPC address for agents")
	fs.StringVar(&cfg.AgentBinaryPath, "agent-binary", cfg.AgentBinaryPath, "path to mxagent binary served for provisioning")
	fs.StringVar(&cfg.AuthToken, "token", cfg.AuthToken, "shared auth token")
	fs.BoolVar(&cfg.AllowedShell, "allow-shell", cfg.AllowedShell, "allow command execution")
	fs.StringVar(&cfg.DefaultNodeAddr, "default-agent-addr", cfg.DefaultNodeAddr, "fallback agent address")
	_ = fs.Parse(args)
	return cfg
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
