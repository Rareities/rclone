package internxt

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// CloudBridge supplies active-network DNS addresses for its CGO-disabled
// executable, which cannot ask Android's native resolver directly.
func init() {
	servers := parseDNSServers(os.Getenv("RCLONE_DNS_SERVERS"))
	if len(servers) == 0 {
		return
	}
	var next atomic.Uint64
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			if network != "udp" && network != "tcp" {
				return nil, errors.New("unsupported DNS transport")
			}
			start := int(next.Add(1)-1) % len(servers)
			var lastErr error
			for i := range servers {
				server := servers[(start+i)%len(servers)]
				conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, server)
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
	}
}

func parseDNSServers(value string) []string {
	var servers []string
	seen := make(map[string]bool)
	for _, token := range strings.Split(value, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		var endpoint string
		if address, err := netip.ParseAddr(strings.Trim(token, "[]")); err == nil {
			endpoint = net.JoinHostPort(address.String(), "53")
		} else if host, port, err := net.SplitHostPort(token); err == nil {
			address, err := netip.ParseAddr(host)
			number, portErr := strconv.Atoi(port)
			if err != nil || portErr != nil || number < 1 || number > 65535 {
				continue
			}
			endpoint = net.JoinHostPort(address.String(), port)
		}
		if endpoint != "" && !seen[endpoint] {
			seen[endpoint] = true
			servers = append(servers, endpoint)
		}
	}
	return servers
}
