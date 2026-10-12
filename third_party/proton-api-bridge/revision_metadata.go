package proton_api_bridge

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rclone/go-proton-api"
	"golang.org/x/sync/singleflight"
)

const revisionMetadataBytes = 8 << 20
const revisionMetadataEntries = 2048

var errRevisionMetadataOwnerCancelled = errors.New("revision metadata request owner cancelled")

// Only signed revision metadata is retained. Names, children and active IDs are
// always read afresh; cache hits are authenticated against the current node key.
type revisionMetadataStore struct {
	mu      sync.Mutex
	entries map[string]proton.RevisionMetadata
	bytes   int
	slots   chan struct{}
	loads   singleflight.Group
}

func revisionMetadataKey(share, link, revision string) string {
	return fmt.Sprintf("%q:%q:%q", share, link, revision)
}

func revisionMetadataCost(key string, metadata proton.RevisionMetadata) int {
	return len(key) + len(metadata.ID) + len(metadata.XAttr) + len(metadata.ManifestSignature) + len(metadata.SignatureEmail) + len(metadata.ThumbnailHash) + 128
}

func (s *revisionMetadataStore) get(key string) (proton.RevisionMetadata, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.entries[key]
	return v, ok
}

func (s *revisionMetadataStore) put(key string, v proton.RevisionMetadata) {
	cost := revisionMetadataCost(key, v)
	if v.ID == "" || v.XAttr == "" || cost > revisionMetadataBytes {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, exists := s.entries[key]
	if exists {
		s.bytes -= revisionMetadataCost(key, old)
	}
	if s.entries == nil || s.bytes+cost > revisionMetadataBytes || (!exists && len(s.entries) >= revisionMetadataEntries) {
		s.entries = make(map[string]proton.RevisionMetadata)
		s.bytes = 0
	}
	s.entries[key] = v
	s.bytes += cost
}

func (s *revisionMetadataStore) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = nil
	s.bytes = 0
}

func (s *revisionMetadataStore) load(ctx context.Context, key string, fetch func() (proton.RevisionMetadata, error)) (proton.RevisionMetadata, error) {
	for {
		if err := ctx.Err(); err != nil {
			return proton.RevisionMetadata{}, err
		}
		if v, ok := s.get(key); ok {
			return v, nil
		}
		result := s.loads.DoChan(key, func() (any, error) {
			if v, ok := s.get(key); ok {
				return v, nil
			}
			s.mu.Lock()
			if s.slots == nil {
				s.slots = make(chan struct{}, 4)
			}
			slots := s.slots
			s.mu.Unlock()
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				return nil, errors.Join(errRevisionMetadataOwnerCancelled, ctx.Err())
			}
			v, err := fetch()
			if ctx.Err() != nil {
				return nil, errors.Join(errRevisionMetadataOwnerCancelled, ctx.Err())
			}
			if err != nil {
				return nil, err
			}
			s.put(key, v)
			return v, nil
		})
		select {
		case <-ctx.Done():
			return proton.RevisionMetadata{}, ctx.Err()
		case r := <-result:
			if errors.Is(r.Err, errRevisionMetadataOwnerCancelled) {
				continue
			}
			if r.Err != nil {
				return proton.RevisionMetadata{}, r.Err
			}
			return r.Val.(proton.RevisionMetadata), nil
		}
	}
}
