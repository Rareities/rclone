package proton_api_bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/rclone/go-proton-api"
)

func signedRevisionAttrs(t *testing.T, drive *ProtonDrive, nodeKR *crypto.KeyRing, digest string) string {
	return signedRevisionAttrsOfSize(t, drive, nodeKR, digest, 29)
}

func signedRevisionAttrsOfSize(t *testing.T, drive *ProtonDrive, nodeKR *crypto.KeyRing, digest string, size int64) string {
	t.Helper()
	blocks := []int64{}
	if size > 0 {
		blocks = append(blocks, size)
	}
	attrs, err := json.Marshal(proton.RevisionXAttr{Common: proton.RevisionXAttrCommon{ModificationTime: "2026-10-11T12:00:00Z", Size: size, BlockSizes: blocks, Digests: map[string]string{"SHA1": digest}}})
	if err != nil {
		t.Fatal(err)
	}
	encrypt, err := crypto.PGP().Encryption().Recipients(nodeKR).SigningKeys(drive.addrKRs["revision"]).SignTime(time.Now().Add(-time.Hour).Unix()).New()
	if err != nil {
		t.Fatal(err)
	}
	message, err := encrypt.Encrypt(attrs)
	if err != nil {
		t.Fatal(err)
	}
	armored, err := message.Armor()
	if err != nil {
		t.Fatal(err)
	}
	return armored
}

func TestListingAttrsPreferExplicitActiveRevision(t *testing.T) {
	fixture := newListingFixture(t, 1)
	link := fixture.children[0]
	nodeKR, err := fixture.drive.getLinkKR(context.Background(), &link)
	if err != nil {
		t.Fatal(err)
	}
	oldHash := "a7838d914162b3d8392878eaad30e1de1713e5e0"
	activeHash := "4f1e4374a22a9e62b2b263b70396eea4b513b30e"
	link.XAttr = signedRevisionAttrs(t, fixture.drive, nodeKR, oldHash)
	link.FileProperties.ActiveRevision.XAttr = signedRevisionAttrs(t, fixture.drive, nodeKR, activeHash)
	fixture.revisionMu.Lock()
	fixture.revisions["/drive/shares/share/files/0/revisions/rev-0"] = link.FileProperties.ActiveRevision
	fixture.revisionMu.Unlock()
	fetched, err := link.FileProperties.ActiveRevision.GetDecXAttrString(fixture.drive.addrKRs["revision"], nodeKR)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := fixture.drive.activeRevisionAttrsWithKey(context.Background(), &link, nodeKR)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("valid signed link attrs=%s; active revision attrs=%s; listing chose=%s", oldHash, fetched.Digests["SHA1"], listed.Digests)
	if listed.Digests != fetched.Digests["SHA1"] {
		t.Fatalf("listing discarded explicit signed active revision metadata: got %s, want %s", listed.Digests, fetched.Digests["SHA1"])
	}
}

func TestListingAttrsRejectInvalidPreferredRevision(t *testing.T) {
	fixture := newListingFixture(t, 1)
	link := fixture.children[0]
	nodeKR, err := fixture.drive.getLinkKR(context.Background(), &link)
	if err != nil {
		t.Fatal(err)
	}
	link.FileProperties.ActiveRevision.XAttr = "invalid preferred revision ciphertext"
	if _, err := fixture.drive.activeRevisionAttrsWithKey(context.Background(), &link, nodeKR); err == nil {
		t.Fatal("invalid explicit active revision metadata was silently replaced by link attrs")
	}
}

func TestActiveRevisionUsesItsOwnSignerAndRejectsChangedIdentity(t *testing.T) {
	fixture := newListingFixture(t, 1)
	link := fixture.children[0]
	nodeKR, err := fixture.drive.getLinkKR(context.Background(), &link)
	if err != nil {
		t.Fatal(err)
	}
	// The source link lacks a revision ID, so the fetched active revision is the
	// authoritative metadata snapshot. Its signer differs from the older link hint.
	link.FileProperties.ActiveRevision.SignatureEmail = "test@example.com"
	link.FileProperties.ActiveRevision.ID = ""
	revision := proton.Revision{RevisionMetadata: proton.RevisionMetadata{ID: "fresh", State: proton.RevisionStateActive, SignatureEmail: "revision@example.com", XAttr: signedRevisionAttrs(t, fixture.drive, nodeKR, "4f1e4374a22a9e62b2b263b70396eea4b513b30e")}}
	activeID := "fresh"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/drive/shares/share/files/0/revisions":
			metadata := revision.RevisionMetadata
			metadata.ID = activeID
			_ = json.NewEncoder(w).Encode(map[string]any{"Code": 1000, "Revisions": []proton.RevisionMetadata{metadata}})
		case "/drive/shares/share/files/0/revisions/fresh":
			_ = json.NewEncoder(w).Encode(map[string]any{"Code": 1000, "Revision": revision})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	defer server.Close()
	manager := proton.New(proton.WithHostURL(server.URL))
	defer manager.Close()
	fixture.drive.c = manager.NewClient("uid", "access", "refresh")
	fixture.drive.cache = newCache(true, nil)
	fixture.drive.cache._insert(link.LinkID, &link, nodeKR)
	_, attrs, err := fixture.drive.GetActiveRevisionWithAttrs(context.Background(), &link)
	if err != nil {
		t.Fatalf("valid fresh revision rejected using stale link signer: %v", err)
	}
	if attrs == nil || attrs.Digests != "4f1e4374a22a9e62b2b263b70396eea4b513b30e" {
		t.Fatalf("unexpected fresh attrs: %+v", attrs)
	}
	link.FileProperties.ActiveRevision.ID = "stale"
	if _, _, err := fixture.drive.GetActiveRevisionWithAttrs(context.Background(), &link); err != nil {
		t.Fatalf("stale listing hint must not override fresh history identity: %v", err)
	}
	if _, _, err := fixture.drive.getActiveRevisionWithAttrs(context.Background(), &link, "stale"); err == nil || !strings.Contains(err.Error(), "active revision changed") {
		t.Fatalf("changed verified snapshot must stop before download: %v", err)
	}
	link.FileProperties.ActiveRevision.ID = "fresh"
	revision.ID = "unrelated"
	if _, _, err := fixture.drive.GetActiveRevisionWithAttrs(context.Background(), &link); err == nil || !strings.Contains(err.Error(), "does not match requested revision") {
		t.Fatalf("wrong revision response must be rejected: %v", err)
	}
	activeID = ""
	if _, _, err := fixture.drive.GetActiveRevisionWithAttrs(context.Background(), &link); err == nil || !strings.Contains(err.Error(), "has no identity") {
		t.Fatalf("missing revision identity must be rejected: %v", err)
	}
}
