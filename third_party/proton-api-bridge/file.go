package proton_api_bridge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ProtonMail/gopenpgp/v3/crypto"

	"github.com/rclone/go-proton-api"
	"github.com/relvacode/iso8601"
)

type FileSystemAttrs struct {
	// RevisionID identifies the revision whose plaintext attributes were verified.
	RevisionID       string
	ModificationTime time.Time
	Size             int64
	BlockSizes       []int64
	Digests          string // sha1 string
}

func (protonDrive *ProtonDrive) GetRevisions(ctx context.Context, link *proton.Link, revisionType proton.RevisionState) ([]*proton.RevisionMetadata, error) {
	revisions, err := protonDrive.c.ListRevisions(ctx, protonDrive.MainShare.ShareID, link.LinkID)
	if err != nil {
		return nil, err
	}

	ret := make([]*proton.RevisionMetadata, 0)
	// Revisions are only for files, they represent “versions” of files.
	// Each file can have 1 active/draft revision and n obsolete revisions.
	for i := range revisions {
		if revisions[i].State == revisionType {
			ret = append(ret, &revisions[i])
		}
	}

	return ret, nil
}

func (protonDrive *ProtonDrive) GetActiveRevisionAttrsByID(ctx context.Context, linkID string) (*FileSystemAttrs, error) {
	link, err := protonDrive.getLink(ctx, linkID)
	if err != nil {
		return nil, err
	}

	return protonDrive.GetActiveRevisionAttrs(ctx, link)
}

// Might return nil when xattr is missing
func (protonDrive *ProtonDrive) GetActiveRevisionAttrs(ctx context.Context, link *proton.Link) (*FileSystemAttrs, error) {
	if link == nil {
		return nil, ErrLinkMustNotBeNil
	}

	nodeKR, err := protonDrive.getLinkKR(ctx, link)
	if err != nil {
		return nil, err
	}
	return protonDrive.activeRevisionAttrsWithKey(ctx, link, nodeKR)
}

// activeRevisionAttrsWithParent uses the verified parent from the same fresh listing.
func (protonDrive *ProtonDrive) activeRevisionAttrsWithParent(ctx context.Context, link *proton.Link, parentKR *crypto.KeyRing) (*FileSystemAttrs, error) {
	verificationKR, err := protonDrive.getSignatureVerificationKeyring([]string{link.SignatureEmail})
	if err != nil {
		return nil, err
	}
	nodeKR, err := link.GetKeyRing(parentKR, verificationKR)
	if err != nil {
		return nil, err
	}
	return protonDrive.activeRevisionAttrsWithKey(ctx, link, nodeKR)
}

func (protonDrive *ProtonDrive) activeRevisionAttrsWithKey(ctx context.Context, link *proton.Link, nodeKR *crypto.KeyRing) (*FileSystemAttrs, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if link.FileProperties == nil {
		return nil, fmt.Errorf("file %s has no revision properties", link.LinkID)
	}
	revisionMetadata := link.FileProperties.ActiveRevision
	if revisionMetadata.XAttr == "" {
		// Legacy links may advertise a stale ID. Resolve the active identity afresh
		// before reusing immutable, signed metadata for that revision.
		var err error
		revisionMetadata, err = protonDrive.revisionMetadata.fresh(ctx,
			revisionMetadataKey(protonDrive.MainShare.ShareID, link.LinkID, ""),
			func() (proton.RevisionMetadata, error) {
				revisions, err := protonDrive.GetRevisions(ctx, link, proton.RevisionStateActive)
				if err != nil {
					return proton.RevisionMetadata{}, err
				}
				if len(revisions) != 1 || revisions[0].ID == "" {
					return proton.RevisionMetadata{}, ErrCantFindActiveRevision
				}
				return *revisions[0], nil
			})
		if err != nil {
			return nil, err
		}
		if revisionMetadata.XAttr == "" {
			key := revisionMetadataKey(protonDrive.MainShare.ShareID, link.LinkID, revisionMetadata.ID)
			if cached, ok := protonDrive.revisionMetadata.get(key); ok {
				revisionMetadata = cached
			}
		}
	}
	attrs, err := protonDrive.revisionAttrsWithKey(revisionMetadata, nodeKR)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err == nil {
		protonDrive.revisionMetadata.put(revisionMetadataKey(protonDrive.MainShare.ShareID, link.LinkID, revisionMetadata.ID), revisionMetadata)
	}
	return attrs, err
}

func (protonDrive *ProtonDrive) revisionAttrsWithKey(revisionMetadata proton.RevisionMetadata, nodeKR *crypto.KeyRing) (*FileSystemAttrs, error) {
	signatureVerificationKR, err := protonDrive.getSignatureVerificationKeyring([]string{revisionMetadata.SignatureEmail})
	if err != nil {
		return nil, err
	}
	revisionXAttrCommon, err := revisionMetadata.GetDecXAttrString(signatureVerificationKR, nodeKR)
	if err != nil {
		return nil, err
	}

	if revisionXAttrCommon == nil {
		return nil, nil
	}

	modificationTime, err := iso8601.ParseString(revisionXAttrCommon.ModificationTime)
	if err != nil {
		return nil, err
	}

	var sha1Hash string
	if val, ok := revisionXAttrCommon.Digests["SHA1"]; ok {
		sha1Hash = strings.ToLower(val) // https://github.com/rclone/Proton-API-Bridge/issues/21 and https://github.com/rclone/rclone/issues/7345#issuecomment-1821463100
	} else {
		sha1Hash = ""
	}

	return &FileSystemAttrs{
		RevisionID:       revisionMetadata.ID,
		ModificationTime: modificationTime,
		Size:             revisionXAttrCommon.Size,
		BlockSizes:       revisionXAttrCommon.BlockSizes,
		Digests:          sha1Hash,
	}, nil
}

func (protonDrive *ProtonDrive) GetActiveRevisionWithAttrs(ctx context.Context, link *proton.Link) (*proton.Revision, *FileSystemAttrs, error) {
	return protonDrive.getActiveRevisionWithAttrs(ctx, link, "")
}

func (protonDrive *ProtonDrive) getActiveRevisionWithAttrs(ctx context.Context, link *proton.Link, expectedRevisionID string) (*proton.Revision, *FileSystemAttrs, error) {
	if link == nil {
		return nil, nil, ErrLinkMustNotBeNil
	}

	revisionsMetadata, err := protonDrive.GetRevisions(ctx, link, proton.RevisionStateActive)
	if err != nil {
		return nil, nil, err
	}

	if len(revisionsMetadata) != 1 {
		return nil, nil, ErrCantFindActiveRevision
	}
	if revisionsMetadata[0].ID == "" {
		return nil, nil, fmt.Errorf("active revision has no identity for file %s", link.LinkID)
	}
	if expectedRevisionID != "" && expectedRevisionID != revisionsMetadata[0].ID {
		return nil, nil, fmt.Errorf("active revision changed while opening file %s", link.LinkID)
	}

	revision, err := protonDrive.c.GetRevisionAllBlocks(ctx, protonDrive.MainShare.ShareID, link.LinkID, revisionsMetadata[0].ID)
	if err != nil {
		return nil, nil, err
	}
	if revision.ID != revisionsMetadata[0].ID {
		return nil, nil, fmt.Errorf("revision response does not match requested revision for file %s", link.LinkID)
	}

	nodeKR, err := protonDrive.getLinkKR(ctx, link)
	if err != nil {
		return nil, nil, err
	}

	attrs, err := protonDrive.revisionAttrsWithKey(revision.RevisionMetadata, nodeKR)
	if err != nil {
		return nil, nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	protonDrive.revisionMetadata.put(revisionMetadataKey(protonDrive.MainShare.ShareID, link.LinkID, revision.ID), revision.RevisionMetadata)
	return &revision, attrs, nil
}
