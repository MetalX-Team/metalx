package discovery

import (
	"context"
	"fmt"
	"net"
	"time"
)

// DiscoverController is a placeholder for future UDP broadcast discovery.
func DiscoverController(ctx context.Context, port int) (string, error) {
	deadline := time.Now().Add(500 * time.Millisecond)
	_ = deadline

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("list interfaces: %w", err)
	}

	for _, addr := range addrs {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() || ipNet.IP.To4() == nil {
			continue
		}
		return fmt.Sprintf("%s:%d", ipNet.IP.String(), port), nil
	}

	return "", fmt.Errorf("no controller discovered")
}
