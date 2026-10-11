package proton_api_bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/rclone/Proton-API-Bridge/common"
	"github.com/rclone/go-proton-api"
)

type listingFixture struct {
	drive          *ProtonDrive
	children       []proton.Link
	folderRequests atomic.Int32
	rootRequests   atomic.Int32
}

// newListingFixture uses real signed names, node keys and attributes.
func newListingFixture(t testing.TB, count int) *listingFixture {
	t.Helper()
	// Backdate fixture signatures so a previously sampled HTTP Date cannot
	// make newly generated signatures appear to come from the future.
	fixtureTime := time.Now().Add(-time.Hour).Unix()
	makeKR := func() *crypto.KeyRing {
		key, err := crypto.PGP().KeyGeneration().GenerationTime(fixtureTime).AddUserId("test", "test@example.com").New().GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		kr, err := crypto.NewKeyRing(key)
		if err != nil {
			t.Fatal(err)
		}
		return kr
	}
	encrypt := func(parent, signer *crypto.KeyRing, plain []byte) string {
		builder := crypto.PGP().Encryption().Recipients(parent).SignTime(fixtureTime)
		if signer != nil {
			builder = builder.SigningKeys(signer)
		}
		handle, err := builder.New()
		if err != nil {
			t.Fatal(err)
		}
		message, err := handle.Encrypt(plain)
		if err != nil {
			t.Fatal(err)
		}
		armored, err := message.Armor()
		if err != nil {
			t.Fatal(err)
		}
		return armored
	}
	signer := makeKR()
	revisionSigner := makeKR()
	share := makeKR()
	makeLink := func(id, parentID, name string, parent *crypto.KeyRing, file bool) (proton.Link, *crypto.KeyRing) {
		t.Helper()
		passphrase := []byte("test node passphrase")
		nodeKR := makeKR()
		lockedKey, err := crypto.PGP().LockKey(nodeKR.GetKeys()[0], passphrase)
		if err != nil {
			t.Fatal(err)
		}
		locked, err := lockedKey.Armor()
		if err != nil {
			t.Fatal(err)
		}
		enc := encrypt(parent, nil, passphrase)
		signHandle, err := crypto.PGP().Sign().SigningKeys(signer).SignTime(fixtureTime).Detached().New()
		if err != nil {
			t.Fatal(err)
		}
		sig, err := signHandle.Sign(passphrase, crypto.Armor)
		if err != nil {
			t.Fatal(err)
		}
		armored := encrypt(parent, signer, []byte(name))
		link := proton.Link{LinkID: id, ParentLinkID: parentID, Type: proton.LinkTypeFolder, State: proton.LinkStateActive,
			Name: armored, NameSignatureEmail: "test@example.com", SignatureEmail: "test@example.com", NodeKey: locked,
			NodePassphrase: enc, NodePassphraseSignature: string(sig)}
		kr, err := link.GetKeyRing(parent, signer)
		if err != nil {
			t.Fatal(err)
		}
		if file {
			link.Type = proton.LinkTypeFile
			link.Size = 999
			link.FileProperties = &proton.FileProperties{ActiveRevision: proton.RevisionMetadata{SignatureEmail: "revision@example.com"}}
			attrs, err := json.Marshal(proton.RevisionXAttr{Common: proton.RevisionXAttrCommon{ModificationTime: "2026-10-11T12:00:00Z", Size: 12, Digests: map[string]string{"SHA1": strings.Repeat("A", 40)}, BlockSizes: []int64{12}}})
			if err != nil {
				t.Fatal(err)
			}
			link.XAttr = encrypt(kr, revisionSigner, attrs)
		}
		return link, kr
	}
	root, rootKR := makeLink("root", "", "root", share, false)
	folder, folderKR := makeLink("folder", "root", "folder", rootKR, false)
	f := &listingFixture{}
	for i := 0; i < count; i++ {
		link, _ := makeLink(fmt.Sprint(i), "folder", fmt.Sprint(i), folderKR, true)
		f.children = append(f.children, link)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/drive/shares/share/links/root":
			f.rootRequests.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"Code": 1000, "Link": root})
		case "/drive/shares/share/links/folder":
			f.folderRequests.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"Code": 1000, "Link": folder})
		case "/drive/shares/share/folders/folder/children":
			links := f.children
			if r.URL.Query().Get("Page") != "0" {
				links = nil
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"Code": 1000, "Links": links})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	m := proton.New(proton.WithHostURL(server.URL))
	t.Cleanup(m.Close)
	config := common.NewConfigWithDefaultValues()
	f.drive = &ProtonDrive{MainShare: &proton.Share{ShareMetadata: proton.ShareMetadata{ShareID: "share"}}, MainShareKR: share, c: m.NewClient("uid", "access", "refresh"),
		cache: newCache(false, config.GetLogger()), addrData: map[string]proton.Address{"test@example.com": {ID: "addr"}, "revision@example.com": {ID: "revision"}}, addrKRs: map[string]*crypto.KeyRing{"addr": signer, "revision": revisionSigner}}
	return f
}

func TestListingAttrsAvoidRepeatedAncestorsAndRemainFresh(t *testing.T) {
	f := newListingFixture(t, 3)
	baseline, err := f.drive.ListDirectory(context.Background(), "folder")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range baseline {
		if _, err := f.drive.GetActiveRevisionAttrs(context.Background(), entry.Link); err != nil {
			t.Fatal(err)
		}
	}
	if f.folderRequests.Load() != 4 || f.rootRequests.Load() != 4 {
		t.Fatalf("unexpected baseline requests: folder=%d root=%d", f.folderRequests.Load(), f.rootRequests.Load())
	}
	t.Logf("three-file uncached baseline: %d ancestor GETs", f.folderRequests.Load()+f.rootRequests.Load())
	f.folderRequests.Store(0)
	f.rootRequests.Store(0)
	entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || f.folderRequests.Load() != 1 || f.rootRequests.Load() != 1 {
		t.Fatalf("listing repeated ancestor fetches: entries=%d folder=%d root=%d", len(entries), f.folderRequests.Load(), f.rootRequests.Load())
	}
	for _, entry := range entries {
		if entry.Attrs == nil || entry.Attrs.Size != 12 || entry.Attrs.Digests != strings.Repeat("a", 40) {
			t.Fatalf("incorrect decrypted attrs: %+v", entry.Attrs)
		}
	}
	f.children = f.children[:1]
	entries, err = f.drive.ListDirectoryWithAttrs(context.Background(), "folder")
	if err != nil || len(entries) != 1 || f.folderRequests.Load() != 2 || f.rootRequests.Load() != 2 {
		t.Fatalf("listing must remain fresh: %v, %v", entries, err)
	}
}

func TestListingAttrsFailsClosedAndPreservesMissingAttrs(t *testing.T) {
	for _, problem := range []string{"wrong parent", "bad node signature", "bad attrs", "wrong revision signer", "bad name", "missing attrs", "missing revision"} {
		t.Run(problem, func(t *testing.T) {
			f := newListingFixture(t, 2)
			switch problem {
			case "wrong parent":
				f.children[1].ParentLinkID = "elsewhere"
			case "bad node signature":
				f.children[1].NodePassphraseSignature = "broken"
			case "missing revision":
				f.children[1].FileProperties = nil
			case "wrong revision signer":
				f.children[1].FileProperties.ActiveRevision.SignatureEmail = "test@example.com"
			case "bad attrs":
				f.children[1].XAttr = "broken"
			case "bad name":
				f.children[1].Name = "broken"
			case "missing attrs":
				f.children[1].XAttr = ""
			}
			entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder")
			if problem == "missing attrs" {
				if err != nil || len(entries) != 2 || entries[1].Attrs != nil {
					t.Fatalf("missing attrs fallback lost: %v, %v", entries, err)
				}
			} else if err == nil || entries != nil {
				t.Fatalf("must reject the whole listing: %v, %v", entries, err)
			}
		})
	}
}

func TestListingAttrsConcurrentAndCancelled(t *testing.T) {
	f := newListingFixture(t, 3)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if entries, err := f.drive.ListDirectoryWithAttrs(ctx, "folder"); err == nil || entries != nil {
		t.Fatalf("cancelled listing must fail: %v %v", entries, err)
	}
	var group sync.WaitGroup
	for range 4 {
		group.Add(1)
		go func() {
			defer group.Done()
			entries, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder")
			if err != nil || len(entries) != 3 {
				t.Errorf("concurrent listing failed: %v %v", entries, err)
			}
		}()
	}
	group.Wait()
}

// BenchmarkDirectoryAttrs compares identical fresh listings with caching disabled.
// No network delay is injected; this measures localhost HTTP plus actual PGP work.
func BenchmarkDirectoryAttrs(b *testing.B) {
	for _, optimized := range []bool{false, true} {
		name := "ancestor-per-file"
		if optimized {
			name = "shared-parent"
		}
		b.Run(name, func(b *testing.B) {
			f := newListingFixture(b, 20)
			b.ResetTimer()
			for range b.N {
				if optimized {
					if _, err := f.drive.ListDirectoryWithAttrs(context.Background(), "folder"); err != nil {
						b.Fatal(err)
					}
				} else {
					entries, err := f.drive.ListDirectory(context.Background(), "folder")
					if err != nil {
						b.Fatal(err)
					}
					for _, entry := range entries {
						if _, err := f.drive.GetActiveRevisionAttrs(context.Background(), entry.Link); err != nil {
							b.Fatal(err)
						}
					}
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(f.folderRequests.Load()+f.rootRequests.Load())/float64(b.N), "ancestor-GET/op")
		})
	}
}
