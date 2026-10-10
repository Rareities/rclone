package fshttp

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
)

func init() {
	// Android's native executable receives its network DNS servers from the app.
	// An absent or invalid override preserves the platform resolver.
	if servers := dnsServerAddresses(os.Getenv("RCLONE_DNS_SERVERS")); len(servers) != 0 {
		net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: dnsServerDialer(servers)}
	}
}

func dnsServerAddresses(value string) []string {
	var servers []string
	seen := make(map[string]bool)
	for _, value := range strings.Split(value, ",") {
		value = strings.TrimSpace(value)
		host, port := value, "53"
		if h, p, err := net.SplitHostPort(value); err == nil {
			host, port = h, p
		} else if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			host = strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
		}
		addr, err := netip.ParseAddr(host)
		if err != nil {
			continue
		}
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			continue
		}
		server := net.JoinHostPort(addr.String(), strconv.Itoa(number))
		if !seen[server] {
			servers = append(servers, server)
			seen[server] = true
		}
	}
	return servers
}

func dnsServerDialer(servers []string) func(context.Context, string, string) (net.Conn, error) {
	var next atomic.Uint64
	return func(ctx context.Context, network, _ string) (net.Conn, error) {
		if len(servers) == 0 {
			return nil, errors.New("no DNS servers configured")
		}
		start := int((next.Add(1) - 1) % uint64(len(servers)))
		var lastErr error
		for i := range servers {
			conn, err := (&net.Dialer{}).DialContext(ctx, network, servers[(start+i)%len(servers)])
			if err == nil {
				return conn, nil
			}
			lastErr = err
			if ctx.Err() != nil {
				break
			}
		}
		return nil, lastErr
	}
}
