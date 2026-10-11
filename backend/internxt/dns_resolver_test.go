package internxt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAndroidDNSServerParsing(t *testing.T) {
	require.Equal(t, []string{"192.0.2.1:53", "[2001:db8::1]:53", "[fe80::1%wlan0]:53", "[2001:db8::2]:5353"},
		parseDNSServers("192.0.2.1,2001:db8::1,fe80::1%wlan0,[2001:db8::2]:5353,192.0.2.1"))
	require.Empty(t, parseDNSServers("hostname.invalid,127.0.0.1:0,127.0.0.1:70000,garbage,"))
	require.Empty(t, parseDNSServers(""))
}
