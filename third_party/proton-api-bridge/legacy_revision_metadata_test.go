package proton_api_bridge

import (
	"context"
	"crypto/sha1"
	"fmt"
	"strings"
	"testing"

	"github.com/rclone/go-proton-api"
)

func TestListingAttrsLegacyRevisionStaysConsistentAfterDownloadAndCommit(t *testing.T) {
	f := newListingFixture(t, 1)
	link := f.children[0]
	nodeKR, err := f.drive.getLinkKR(context.Background(), &link)
	if err != nil {
		t.Fatal(err)
	}
	stale := "a7838d914162b3d8392878eaad30e1de1713e5e0"
	want := "4f1e4374a22a9e62b2b263b70396eea4b513b30e"
	link.XAttr = signedRevisionAttrs(t, f.drive, nodeKR, stale)
	link.FileProperties.ActiveRevision.XAttr = ""
	f.children[0] = link
	path := "/drive/shares/share/files/0/revisions/" + link.FileProperties.ActiveRevision.ID
	f.revisionMu.Lock()
	metadata := f.revisions[path]
	metadata.XAttr = signedRevisionAttrs(t, f.drive, nodeKR, want)
	f.revisions[path] = metadata
	f.revisionMu.Unlock()
	checkListing := func(wantHash string) {
		t.Helper()
		entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder")
		if err != nil || len(entries) != 1 || entries[0].Attrs == nil || entries[0].Attrs.Digests != wantHash {
			t.Fatalf("legacy listing returned stale link checksum: entries=%+v err=%v want=%s", entries, err, wantHash)
		}
	}
	checkListing(want)
	if f.revisionRequests.Load() != 1 {
		t.Fatalf("first legacy scan should fetch one revision: %d", f.revisionRequests.Load())
	}
	checkListing(want)
	if f.revisionRequests.Load() != 1 {
		t.Fatal("unchanged revision was fetched again")
	}
	// Download metadata and later listings must refer to the same authenticated revision.
	f.drive.ClearCache()
	_, attrs, err := f.drive.GetActiveRevisionWithAttrs(context.Background(), &link)
	if err != nil || attrs == nil || attrs.Digests != want {
		t.Fatalf("download metadata inconsistent: %+v %v", attrs, err)
	}
	afterDownload := f.revisionRequests.Load()
	checkListing(want)
	if f.revisionRequests.Load() != afterDownload {
		t.Fatal("download did not seed authenticated metadata reuse")
	}
	// A new revision ID must never reuse the previous revision's checksum.
	next := "next-revision"
	nextHash := fmt.Sprintf("%x", sha1.Sum([]byte(strings.Repeat("C", 29))))
	link.FileProperties.ActiveRevision.ID = next
	f.children[0] = link
	f.revisionMu.Lock()
	f.revisions["/drive/shares/share/files/0/revisions/"+next] = proton.RevisionMetadata{ID: next, State: proton.RevisionStateActive, SignatureEmail: "revision@example.com", XAttr: signedRevisionAttrs(t, f.drive, nodeKR, nextHash)}
	f.revisionMu.Unlock()
	checkListing(nextHash)
	if f.revisionRequests.Load() != afterDownload+1 {
		t.Fatal("changed revision was not fetched")
	}
	// A successful signed upload commit supplies authoritative metadata without a GET.
	committed := "committed-revision"
	committedHash := fmt.Sprintf("%x", sha1.Sum([]byte(strings.Repeat("D", 29))))
	f.drive.DefaultAddrKR = f.drive.addrKRs["revision"]
	f.drive.signatureAddress = "revision@example.com"
	err = f.drive.commitNewRevision(context.Background(), nodeKR, &proton.RevisionXAttrCommon{ModificationTime: "2026-10-11T12:00:00Z", Size: 29, BlockSizes: []int64{29}, Digests: map[string]string{"SHA1": committedHash}}, []byte("signed manifest"), link.LinkID, committed)
	if err != nil {
		t.Fatal(err)
	}
	beforeCommitListing := f.revisionRequests.Load()
	link.FileProperties.ActiveRevision.ID = committed
	f.children[0] = link
	checkListing(committedHash)
	if f.revisionRequests.Load() != beforeCommitListing {
		t.Fatal("committed revision was fetched unnecessarily")
	}
	// Cached ciphertext must still verify using the current signer keys.
	validSigner := f.drive.addrKRs["revision"]
	f.drive.addrKRs["revision"] = f.drive.addrKRs["addr"]
	if entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder"); err == nil || entries != nil {
		t.Fatal("cached metadata bypassed current signature verification")
	}
	f.drive.addrKRs["revision"] = validSigner
	// Preferred but corrupted attributes must fail even when a valid cache record exists.
	f.children[0].FileProperties.ActiveRevision.XAttr = "corrupted preferred attributes"
	if entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder"); err == nil || entries != nil {
		t.Fatal("cache bypassed corrupted preferred attributes")
	}
}

func TestListingAttrsLegacyMissingIdentityRequiresOneActiveRevision(t *testing.T) {
	f := newListingFixture(t, 1)
	f.children[0].FileProperties.ActiveRevision.ID = ""
	f.children[0].FileProperties.ActiveRevision.XAttr = ""
	entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder")
	if err != nil || len(entries) != 1 || entries[0].Attrs == nil {
		t.Fatalf("single identified active revision should supply metadata: %v", err)
	}
	f.revisionMu.Lock()
	metadata := f.revisions["/drive/shares/share/files/0/revisions/rev-0"]
	metadata.ID = "other-active"
	f.revisions["/drive/shares/share/files/0/revisions/other-active"] = metadata
	f.revisionMu.Unlock()
	if entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder"); err == nil || entries != nil {
		t.Fatal("ambiguous active revisions accepted")
	}
}

func TestListingAttrsLegacyRejectsWrongRevisionResponse(t *testing.T) {
	f := newListingFixture(t, 1)
	f.children[0].FileProperties.ActiveRevision.XAttr = ""
	path := "/drive/shares/share/files/0/revisions/rev-0"
	f.revisionMu.Lock()
	metadata := f.revisions[path]
	metadata.ID = "unrelated"
	f.revisions[path] = metadata
	f.revisionMu.Unlock()
	if entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder"); err == nil || entries != nil {
		t.Fatal("wrong revision identity accepted")
	}
}
