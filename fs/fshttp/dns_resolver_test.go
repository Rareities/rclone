package fshttp

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDNSServerAddresses(t *testing.T) {
	assert.Equal(t, []string{"192.0.2.1:53", "[2001:db8::1]:53", "[::1]:5353", "[fe80::1%wlan0]:53"}, dnsServerAddresses(" 192.0.2.1, 2001:db8::1, [::1]:5353, fe80::1%wlan0,192.0.2.1:53 "))
	assert.Empty(t, dnsServerAddresses(" , dns.example, 127.0.0.1:0, [::1]:65536, 127.0.0.1:bad"))
}

func TestDNSServerDialerPreservesTCPAndRotates(t *testing.T) {
	first, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer first.Close()
	second, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer second.Close()
	dial := dnsServerDialer([]string{first.Addr().String(), second.Addr().String()})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, expected := range []net.Listener{first, second, first} {
		conn, err := dial(ctx, "tcp", "unused.invalid:53")
		require.NoError(t, err)
		assert.Equal(t, expected.Addr().String(), conn.RemoteAddr().String())
		require.NoError(t, conn.Close())
	}
}

func TestDNSServerDialerCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := dnsServerDialer([]string{"127.0.0.1:53"})(ctx, "tcp", "unused")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
