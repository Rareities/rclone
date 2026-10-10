package proton_test

import (
	"github.com/rclone/go-proton-api"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNameFixturesRejectUnauthenticatedSignatures(t *testing.T) {
	signer := newTestKeyRing(t, false)
	wrongSigner := newTestKeyRing(t, false)
	for _, v6 := range []bool{false, true} {
		parent := newTestKeyRing(t, v6)
		var req proton.CreateFileReq
		require.NoError(t, req.SetName("é-report.txt", signer, parent))
		link := proton.Link{Name: req.Name}
		name, err := link.GetName(parent, signer)
		require.NoError(t, err)
		require.Equal(t, "é-report.txt", name)
		_, err = link.GetName(parent, wrongSigner)
		require.Error(t, err, "incorrect verification keys must not be accepted")
		unsigned, err := proton.EncryptMessageNonAead(parent, []byte("é-report.txt"), nil)
		require.NoError(t, err)
		armored, err := unsigned.Armor()
		require.NoError(t, err)
		_, err = (proton.Link{Name: armored}).GetName(parent, signer)
		require.Error(t, err, "unsigned encrypted names must not be accepted")
	}
}
