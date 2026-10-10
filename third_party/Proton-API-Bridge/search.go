package proton_api_bridge

import (
	"context"
	"errors"

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

	// use available hash to check if it exists
	// more efficient than linear scan to just do existence check
	// used in rclone when Put(), it will try to see if the object exists or not
	res, err := protonDrive.c.CheckAvailableHashes(ctx, protonDrive.MainShare.ShareID, folderLink.LinkID, proton.CheckAvailableHashesReq{
		Hashes: []string{targetNameHash},
	})
	if err != nil {
		return nil, err
	}

	// Modern clients may use a different hash. An available legacy hash does
	// not prove that the decrypted name is absent.
	_ = res

	childrenLinks, err := protonDrive.c.ListChildren(ctx, protonDrive.MainShare.ShareID, folderLink.LinkID, true)
	if err != nil {
		return nil, err
	}
	return findChildByName(childrenLinks, targetNameHash, targetName, searchForFile, searchForFolder, targetState, func(child *proton.Link) (string, error) {
		keys, err := protonDrive.getSignatureVerificationKeyring([]string{child.NameSignatureEmail, child.SignatureEmail})
		if err != nil {
			return "", err
		}
		return child.GetName(folderLinkKR, keys)
	})
}

func findChildByName(children []proton.Link, targetHash, targetName string, file, folder bool, state proton.LinkState, decryptName func(*proton.Link) (string, error)) (*proton.Link, error) {
	eligible := func(child *proton.Link) bool {
		return child.State == state && ((file && child.Type == proton.LinkTypeFile) || (folder && child.Type == proton.LinkTypeFolder))
	}
	var found *proton.Link
	// Hash matches are checked first, but mixed hash generations still require
	// authenticated names for every eligible sibling to detect ambiguity.
	for _, hashMatch := range []bool{true, false} {
		for i := range children {
			child := &children[i]
			if !eligible(child) || (child.Hash == targetHash) != hashMatch {
				continue
			}
			name, err := decryptName(child)
			if err != nil {
				return nil, err
			}
			if name != targetName {
				continue
			}
			if found != nil && found.LinkID != child.LinkID {
				return nil, errors.New("ambiguous decrypted Proton Drive name")
			}
			found = child
		}
	}
	return found, nil
}
