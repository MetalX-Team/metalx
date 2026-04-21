package discovery

import "fmt"

func Banner(port int) string {
	return fmt.Sprintf("udp discovery planned on :%d", port)
}
