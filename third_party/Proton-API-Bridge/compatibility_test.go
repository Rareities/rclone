package proton_api_bridge

import (
	"context"
	"errors"
	"testing"

	"github.com/rclone/Proton-API-Bridge/common"
	"github.com/rclone/go-proton-api"
	"github.com/stretchr/testify/require"
)

func TestModernNameLookupStateTypeAndAmbiguity(t *testing.T) {
	children := []proton.Link{
		{LinkID: "folder", Type: proton.LinkTypeFolder, State: proton.LinkStateActive, Hash: "modern"},
		{LinkID: "draft", Type: proton.LinkTypeFile, State: proton.LinkStateDraft, Hash: "modern"},
		{LinkID: "file", Type: proton.LinkTypeFile, State: proton.LinkStateActive, Hash: "modern"},
	}
	decrypt := func(*proton.Link) (string, error) { return "é.txt", nil }
	link, err := findChildByName(children, "legacy", "é.txt", true, false, proton.LinkStateActive, decrypt)
	require.NoError(t, err)
	require.Equal(t, "file", link.LinkID)
	link, err = findChildByName(children, "legacy", "é.txt", true, false, proton.LinkStateDraft, decrypt)
	require.NoError(t, err)
	require.Equal(t, "draft", link.LinkID)
	_, err = findChildByName(children, "legacy", "é.txt", true, false, proton.LinkStateActive, func(*proton.Link) (string, error) { return "", errors.New("invalid signature") })
	require.ErrorContains(t, err, "invalid signature")
	children = append(children, proton.Link{LinkID: "duplicate", Type: proton.LinkTypeFile, State: proton.LinkStateActive, Hash: "other-modern"})
	_, err = findChildByName(children, "legacy", "é.txt", true, false, proton.LinkStateActive, decrypt)
	require.ErrorContains(t, err, "ambiguous")
}

func TestDraftReplacementFailsClosedWithoutOwnership(t *testing.T) {
	drive := &ProtonDrive{Config: &common.Config{ReplaceExistingDraft: true}}
	// A nil API client ensures no deletion request can precede ownership validation.
	_, retry, err := drive.handleRevisionConflict(context.Background(), &proton.Link{State: proton.LinkStateDraft}, nil)
	require.ErrorIs(t, err, ErrDraftExists)
	require.False(t, retry)
}

func TestMixedLegacyModernNameCollisionIsAmbiguous(t *testing.T) {
	children := []proton.Link{
		{LinkID: "legacy", Type: proton.LinkTypeFile, State: proton.LinkStateActive, Hash: "target-hash"},
		{LinkID: "modern", Type: proton.LinkTypeFile, State: proton.LinkStateActive, Hash: "modern-hash"},
	}
	_, err := findChildByName(children, "target-hash", "same.txt", true, false, proton.LinkStateActive, func(*proton.Link) (string, error) { return "same.txt", nil })
	require.ErrorContains(t, err, "ambiguous")
}
