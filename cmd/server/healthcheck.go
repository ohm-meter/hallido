package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"sonos-radio-agent/internal/config"
)

// runHealthcheck asks the running agent for its web app page on the loopback
// interface. Used as the container HEALTHCHECK: the image has no shell or
// curl. Exit code 0 = healthy, 1 = not healthy. The page is served without
// token and without speaker access, so the check never causes LAN traffic.
func runHealthcheck(cfgPath string) int {
	url := "http://" + net.JoinHostPort("127.0.0.1", listenPort(cfgPath)) + "/"
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "unhealthy:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "unhealthy: HTTP", resp.StatusCode)
		return 1
	}
	return 0
}

// listenPort: SRA_LISTEN, else the config's "listen", else 8080.
func listenPort(cfgPath string) string {
	listen := os.Getenv("SRA_LISTEN")
	if listen == "" {
		if cfg, err := config.ReadFile(cfgPath); err == nil {
			listen = cfg.Listen
		}
	}
	if _, port, err := net.SplitHostPort(listen); err == nil && port != "" {
		return port
	}
	return "8080"
}
