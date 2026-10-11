package proton_api_bridge

import (
	"context"
	"fmt"

	"github.com/rclone/go-proton-api"
)

/*
The filename is unique in a given folder, since it's checked (by using hash) on the server
*/

// if the target isn't found, nil will be returned for both return values
func (protonDrive *ProtonDrive) SearchByNameInActiveFolderByID(ctx context.Context,
	folderLinkID string,
	targetName string,
	searchForFile, searchForFolder bool,
	targetState proton.LinkState) (*proton.Link, error) {
	folderLink, err := protonDrive.getLink(ctx, folderLinkID)
	if err != nil {
		return nil, err
	}

	return protonDrive.SearchByNameInActiveFolder(ctx, folderLink, targetName, searchForFile, searchForFolder, targetState)
}

func (protonDrive *ProtonDrive) SearchByNameInActiveFolder(
	ctx context.Context,
	folderLink *proton.Link,
	targetName string,
	searchForFile, searchForFolder bool,
	targetState proton.LinkState) (*proton.Link, error) {
	if !searchForFile && !searchForFolder {
		// nothing to search
		return nil, nil
	}

	// we search all folders and files within this designated folder only
	if folderLink.Type != proton.LinkTypeFolder {
		return nil, ErrLinkTypeMustToBeFolderType
	}

	if folderLink.State != proton.LinkStateActive {
		// we only search in the active folders
		return nil, nil
	}

	// get target name Hash
	parentNodeKR, err := protonDrive.getLinkKRByID(ctx, folderLink.ParentLinkID)
	if err != nil {
		return nil, err
	}

	signatureVerificationKR, err := protonDrive.getSignatureVerificationKeyring([]string{folderLink.SignatureEmail})
	if err != nil {
		return nil, err
	}
	folderLinkKR, err := folderLink.GetKeyRing(parentNodeKR, signatureVerificationKR)
	if err != nil {
		return nil, err
	}

	signatureVerificationKR, err = protonDrive.getSignatureVerificationKeyring([]string{folderLink.SignatureEmail}, folderLinkKR)
	if err != nil {
		return nil, err
	}
	folderHashKey, err := folderLink.GetHashKey(folderLinkKR, signatureVerificationKR)
	if err != nil {
		return nil, err
	}

	targetNameHash, err := proton.GetNameHash(targetName, folderHashKey)
	if err != nil {
		return nil, err
	}

	// Availability of the legacy hash does not establish absence: modern
	// clients can use a different hash for the same authenticated plaintext name.
	childrenLinks, err := protonDrive.c.ListChildren(ctx, protonDrive.MainShare.ShareID, folderLink.LinkID, true)
	if err != nil {
		return nil, err
	}
	return findAuthenticatedName(childrenLinks, targetNameHash, targetName, searchForFile, searchForFolder, targetState,
		func(link *proton.Link) (string, error) {
			verificationKR, err := protonDrive.getSignatureVerificationKeyring([]string{link.NameSignatureEmail, link.SignatureEmail})
			if err != nil {
				return "", err
			}
			return link.GetName(folderLinkKR, verificationKR)
		})
}

// Both normal lookup and upload conflict handling call this path. Names are
// compared exactly after authenticated decryption; signature failures stay fatal.
func findAuthenticatedName(children []proton.Link, targetHash, targetName string,
	wantFile, wantFolder bool, state proton.LinkState,
	decrypt func(*proton.Link) (string, error)) (*proton.Link, error) {
	var match *proton.Link
	for i := range children {
		child := &children[i]
		if child.State != state {
			continue
		}
		name, err := decrypt(child)
		if err != nil {
			return nil, fmt.Errorf("cannot authenticate a filename during lookup: %w", err)
		}
		if name != targetName {
			if child.Hash == targetHash {
				return nil, fmt.Errorf("filename hash and authenticated name disagree")
			}
			continue
		}
		if !((child.Type == proton.LinkTypeFile && wantFile) || (child.Type == proton.LinkTypeFolder && wantFolder)) {
			return nil, fmt.Errorf("a different object type already occupies this filename")
		}
		if match != nil && match.LinkID != child.LinkID {
			return nil, fmt.Errorf("ambiguous authenticated filename")
		}
		// A legacy hash match cannot exclude a second modern-hash object with
		// the same plaintext name. Validate the entire candidate set before use.
		match = child
	}
	return match, nil
}
