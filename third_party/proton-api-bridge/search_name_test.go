package proton_api_bridge

import (
	"errors"
	"testing"

	"github.com/rclone/go-proton-api"
)

func TestAuthenticatedNameModernHashAndFastPath(t *testing.T) {
	for _, hash := range []string{"legacy", "modern"} {
		links := []proton.Link{{LinkID: "id", Hash: hash, Type: proton.LinkTypeFile, State: proton.LinkStateActive}}
		link, err := findAuthenticatedName(links, "legacy", "file", true, false, proton.LinkStateActive,
			func(*proton.Link) (string, error) { return "file", nil })
		if err != nil || link == nil || link.LinkID != "id" {
			t.Fatalf("lookup failed: %v, %v", link, err)
		}
	}
}

func TestAuthenticatedNameRejectsAmbiguityTypeCollisionAndBadSignatures(t *testing.T) {
	links := []proton.Link{
		{LinkID: "a", Hash: "new1", Type: proton.LinkTypeFile, State: proton.LinkStateActive},
		{LinkID: "b", Hash: "new2", Type: proton.LinkTypeFile, State: proton.LinkStateActive},
	}
	decrypt := func(*proton.Link) (string, error) { return "file", nil }
	if _, err := findAuthenticatedName(links, "legacy", "file", true, false, proton.LinkStateActive, decrypt); err == nil {
		t.Fatal("duplicate authenticated names must not choose an arbitrary file")
	}
	links = links[:1]
	links[0].Type = proton.LinkTypeFolder
	if _, err := findAuthenticatedName(links, "legacy", "file", true, false, proton.LinkStateActive, decrypt); err == nil {
		t.Fatal("file/folder collision must not be treated as absence")
	}
	badSignature := errors.New("signature verification failed")
	if _, err := findAuthenticatedName(links, "legacy", "file", true, true, proton.LinkStateActive,
		func(*proton.Link) (string, error) { return "", badSignature }); !errors.Is(err, badSignature) {
		t.Fatalf("signature failure was lost: %v", err)
	}
}

func TestAuthenticatedNameKeepsDraftAndActiveStatesSeparate(t *testing.T) {
	links := []proton.Link{{LinkID: "draft", Hash: "modern", Type: proton.LinkTypeFile, State: proton.LinkStateDraft}}
	decrypt := func(*proton.Link) (string, error) { return "name", nil }
	link, err := findAuthenticatedName(links, "legacy", "name", true, false, proton.LinkStateActive, decrypt)
	if err != nil || link != nil {
		t.Fatalf("draft was reported active: %v, %v", link, err)
	}
	link, err = findAuthenticatedName(links, "legacy", "name", true, false, proton.LinkStateDraft, decrypt)
	if err != nil || link == nil {
		t.Fatalf("draft lookup failed: %v, %v", link, err)
	}
}

func TestAuthenticatedNameRejectsMixedHashDuplicate(t *testing.T) {
	links := []proton.Link{
		{LinkID: "legacy-id", Hash: "legacy", Type: proton.LinkTypeFile, State: proton.LinkStateActive},
		{LinkID: "modern-id", Hash: "modern", Type: proton.LinkTypeFile, State: proton.LinkStateActive},
	}
	_, err := findAuthenticatedName(links, "legacy", "file", true, false, proton.LinkStateActive,
		func(*proton.Link) (string, error) { return "file", nil })
	if err == nil {
		t.Fatal("a legacy hash match must not hide a modern object with the same authenticated name")
	}
}
