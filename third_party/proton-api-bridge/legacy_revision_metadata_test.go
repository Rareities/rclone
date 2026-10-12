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
	if f.revisionRequests.Load() != 2 {
		t.Fatalf("first legacy scan should resolve identity and detail: %d", f.revisionRequests.Load())
	}
	checkListing(want)
	if f.revisionRequests.Load() != 3 {
		t.Fatal("active identity was not refreshed")
	}
	// Download metadata and later listings must refer to the same authenticated revision.
	f.drive.ClearCache()
	_, attrs, err := f.drive.GetActiveRevisionWithAttrs(context.Background(), &link)
	if err != nil || attrs == nil || attrs.Digests != want {
		t.Fatalf("download metadata inconsistent: %+v %v", attrs, err)
	}
	afterDownload := f.revisionRequests.Load()
	checkListing(want)
	if f.revisionRequests.Load() != afterDownload+1 {
		t.Fatal("download listing did not refresh active identity")
	}
	// A new revision ID must never reuse the previous revision's checksum.
	next := "next-revision"
	nextHash := fmt.Sprintf("%x", sha1.Sum([]byte(strings.Repeat("C", 29))))
	// Keep the advertised hint stuck at the old ID while history advances.
	f.children[0] = link
	f.revisionMu.Lock()
	previous := f.revisions[path]
	previous.State = proton.RevisionStateObsolete
	f.revisions[path] = previous
	f.revisions["/drive/shares/share/files/0/revisions/"+next] = proton.RevisionMetadata{ID: next, State: proton.RevisionStateActive, SignatureEmail: "revision@example.com", XAttr: signedRevisionAttrs(t, f.drive, nodeKR, nextHash)}
	f.revisionMu.Unlock()
	checkListing(nextHash)
	beforeOpen := f.revisionRequests.Load()
	if _, _, err := f.drive.getActiveRevisionWithAttrs(context.Background(), &link, "rev-0"); err == nil || !strings.Contains(err.Error(), "active revision changed") {
		t.Fatalf("changed verified snapshot accepted: %v", err)
	}
	if f.revisionRequests.Load() != beforeOpen+1 {
		t.Fatal("snapshot mismatch fetched detailed revision before failing")
	}
	if f.revisionRequests.Load() != afterDownload+4 {
		t.Fatal("changed revision was not fetched")
	}
	// A successful commit still requires fresh active identity on later listings.
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
	if f.revisionRequests.Load() != beforeCommitListing+1 {
		t.Fatal("committed revision identity was not refreshed")
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

func TestListingAttrsLegacyRejectsMissingRevisionIdentity(t *testing.T) {
	f := newListingFixture(t, 1)
	f.children[0].FileProperties.ActiveRevision.XAttr = ""
	path := "/drive/shares/share/files/0/revisions/rev-0"
	f.revisionMu.Lock()
	metadata := f.revisions[path]
	metadata.ID = ""
	f.revisions[path] = metadata
	f.revisionMu.Unlock()
	if entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder"); err == nil || entries != nil {
		t.Fatal("wrong revision identity accepted")
	}
}

func TestListingAttrsLegacyHandlesRejectedRangeAndEmptyFile(t *testing.T) {
	f := newListingFixture(t, 1)
	link := f.children[0]
	nodeKR, err := f.drive.getLinkKR(context.Background(), &link)
	if err != nil {
		t.Fatal(err)
	}
	link.FileProperties.ActiveRevision.XAttr = ""
	f.children[0] = link
	want := fmt.Sprintf("%x", sha1.Sum(nil))
	f.revisionMu.Lock()
	f.rejectMetadataRange = true
	path := "/drive/shares/share/files/0/revisions/rev-0"
	metadata := f.revisions[path]
	metadata.XAttr = signedRevisionAttrsOfSize(t, f.drive, nodeKR, want, 0)
	f.revisions[path] = metadata
	f.revisionMu.Unlock()
	entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder")
	if err != nil || len(entries) != 1 || entries[0].Attrs == nil || entries[0].Attrs.Size != 0 || entries[0].Attrs.Digests != want {
		t.Fatalf("empty file must use signed revision metadata without rejected block-range request: %v", err)
	}
}

func TestListingAttrsLegacyHistoryFailureCannotUseWarmCache(t *testing.T) {
	f := newListingFixture(t, 1)
	f.children[0].FileProperties.ActiveRevision.XAttr = ""
	if _, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder"); err != nil {
		t.Fatal(err)
	}
	f.revisionMu.Lock()
	f.rejectRevisionHistory = true
	f.revisionMu.Unlock()
	if entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder"); err == nil || entries != nil {
		t.Fatal("history failure was concealed by cached metadata")
	}
}

func TestListingAttrsLegacyMissingHistoryAttrsUsesOnlyCanonicalCache(t *testing.T) {
	f := newListingFixture(t, 1)
	f.children[0].FileProperties.ActiveRevision.XAttr = ""
	entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder")
	if err != nil || len(entries) != 1 || entries[0].Attrs == nil {
		t.Fatalf("signed history scan failed: %v", err)
	}
	want := entries[0].Attrs.Digests
	f.revisionMu.Lock()
	path := "/drive/shares/share/files/0/revisions/rev-0"
	metadata := f.revisions[path]
	metadata.XAttr = ""
	f.revisions[path] = metadata
	f.revisionMu.Unlock()
	entries, err = f.drive.ListDirectoryWithAttrs(context.Background(), "folder")
	if err != nil || entries[0].Attrs == nil || entries[0].Attrs.RevisionID != "rev-0" || entries[0].Attrs.Digests != want {
		t.Fatalf("canonical signed cache not reusable: %v", err)
	}
	f.revisionMu.Lock()
	delete(f.revisions, path)
	metadata.ID = "next"
	f.revisions["/drive/shares/share/files/0/revisions/next"] = metadata
	f.revisionMu.Unlock()
	entries, err = f.drive.ListDirectoryWithAttrs(context.Background(), "folder")
	if err != nil || entries[0].Attrs != nil {
		t.Fatalf("old cached attributes supplied for new revision: %v", err)
	}
}

func TestListingAttrsLegacyInvalidHistoryCannotUseWarmCache(t *testing.T) {
	f := newListingFixture(t, 1)
	f.children[0].FileProperties.ActiveRevision.XAttr = ""
	if _, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder"); err != nil {
		t.Fatal(err)
	}
	f.revisionMu.Lock()
	path := "/drive/shares/share/files/0/revisions/rev-0"
	metadata := f.revisions[path]
	metadata.XAttr = "corrupted history attributes"
	f.revisions[path] = metadata
	f.revisionMu.Unlock()
	if entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder"); err == nil || entries != nil {
		t.Fatal("invalid history metadata concealed by valid cache")
	}
}

func TestListingAttrsUsesDetailedRevisionBeforeTransfer(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("legacy=%v", legacy), func(t *testing.T) {
			f := newListingFixture(t, 1)
			link := f.children[0]
			nodeKR, err := f.drive.getLinkKR(context.Background(), &link)
			if err != nil {
				t.Fatal(err)
			}
			content := []byte("current authenticated content")
			want := fmt.Sprintf("%x", sha1.Sum(content))
			older := fmt.Sprintf("%x", sha1.Sum([]byte("older authenticated content!!")))
			path := "/drive/shares/share/files/0/revisions/rev-0"
			f.revisionMu.Lock()
			history := f.revisions[path]
			history.XAttr = signedRevisionAttrsOfSize(t, f.drive, nodeKR, older, int64(len(content)))
			f.revisions[path] = history
			detail := history
			detail.XAttr = signedRevisionAttrsOfSize(t, f.drive, nodeKR, want, int64(len(content)))
			f.detailedOverrides = map[string]proton.RevisionMetadata{path: detail}
			f.revisionMu.Unlock()
			link.FileProperties.ActiveRevision = history
			if legacy {
				link.FileProperties.ActiveRevision.XAttr = ""
			}
			f.children[0] = link
			entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder")
			if err != nil || len(entries) != 1 || entries[0].Attrs == nil || entries[0].Attrs.Digests != want {
				t.Fatalf("bisync must capture detailed revision checksum before transfer: entries=%+v err=%v want=%s", entries, err, want)
			}
			_, attrs, err := f.drive.GetActiveRevisionWithAttrs(context.Background(), &link)
			if err != nil || attrs == nil || attrs.Digests != entries[0].Attrs.Digests || attrs.RevisionID != entries[0].Attrs.RevisionID {
				t.Fatalf("listing/download snapshot inconsistent: %+v %v", attrs, err)
			}
		})
	}
}

func TestListingAttrsRejectsInvalidDetailedRevision(t *testing.T) {
	for _, invalid := range []string{"signature", "wrong identity", "missing identity"} {
		t.Run(invalid, func(t *testing.T) {
			f := newListingFixture(t, 1)
			path := "/drive/shares/share/files/0/revisions/rev-0"
			f.revisionMu.Lock()
			metadata := f.revisions[path]
			switch invalid {
			case "signature":
				metadata.XAttr = "invalid detailed attributes"
			case "wrong identity":
				metadata.ID = "unrelated"
			case "missing identity":
				metadata.ID = ""
			}
			f.detailedOverrides = map[string]proton.RevisionMetadata{path: metadata}
			f.revisionMu.Unlock()
			if entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder"); err == nil || entries != nil {
				t.Fatal("invalid detail concealed by valid advertised attributes")
			}
		})
	}
}

func TestListingAttrsDetailedCacheRevalidatesSignerWithoutHistoryAttrs(t *testing.T) {
	f := newListingFixture(t, 1)
	f.children[0].FileProperties.ActiveRevision.XAttr = ""
	if _, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder"); err != nil {
		t.Fatal(err)
	}
	f.revisionMu.Lock()
	path := "/drive/shares/share/files/0/revisions/rev-0"
	metadata := f.revisions[path]
	metadata.XAttr = ""
	f.revisions[path] = metadata
	f.revisionMu.Unlock()
	f.drive.addrKRs["revision"] = f.drive.addrKRs["addr"]
	if entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder"); err == nil || entries != nil {
		t.Fatal("cached detail bypassed current signature verification")
	}
}
