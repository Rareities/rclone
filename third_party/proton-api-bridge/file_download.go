package proton_api_bridge

import (
	"bytes"
	"context"
	"io"
	"os"
	"sync/atomic"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
	"github.com/rclone/Proton-API-Bridge/utility"
	"github.com/rclone/go-proton-api"
)

type FileDownloadReader struct {
	protonDrive *ProtonDrive
	ctx         context.Context
	cancel      context.CancelFunc
	closed      atomic.Bool

	link         *proton.Link
	data         *bytes.Buffer
	nodeKR       *crypto.KeyRing
	sessionKey   *crypto.SessionKey
	revision     *proton.Revision
	nextRevision int

	isEOF bool

	// TODO: integrity check if the entire file is read
}

func (r *FileDownloadReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.closed.Load() {
		return 0, os.ErrClosed
	}
	if r.data.Len() == 0 {
		// TODO: do we have memory sharing bug?
		// to avoid sharing the underlying buffer array across re-population
		r.data = bytes.NewBuffer(nil)

		// we download and decrypt more content
		err := r.populateBufferOnRead()
		if err != nil {
			return 0, err
		}

		if r.isEOF {
			// if the file has been downloaded entirely, we return EOF
			return 0, io.EOF
		}
	}

	return r.data.Read(p)
}

func (r *FileDownloadReader) Close() error {
	if !r.closed.Swap(true) && r.cancel != nil {
		r.cancel()
	}
	return nil
}

func (reader *FileDownloadReader) populateBufferOnRead() (err error) {
	if len(reader.revision.Blocks) == 0 || len(reader.revision.Blocks) == reader.nextRevision {
		reader.isEOF = true
		return nil
	}

	offset := reader.nextRevision
	for i := offset; i-offset < DOWNLOAD_BATCH_BLOCK_SIZE && i < len(reader.revision.Blocks); i++ {
		if reader.closed.Load() {
			return os.ErrClosed
		}
		// TODO: parallel download
		var blockReader io.ReadCloser
		blockReader, err = reader.protonDrive.c.GetBlock(reader.ctx, reader.revision.Blocks[i].BareURL, reader.revision.Blocks[i].Token)
		if err != nil {
			return err
		}
		defer utility.CheckClose(blockReader, &err)

		var signatureVerificationKR *crypto.KeyRing
		signatureVerificationKR, err = reader.protonDrive.getSignatureVerificationKeyring([]string{reader.link.SignatureEmail}, reader.nodeKR)
		if err != nil {
			return err
		}
		err = decryptBlockIntoBuffer(reader.sessionKey, signatureVerificationKR, reader.nodeKR, reader.revision.Blocks[i].Hash, reader.revision.Blocks[i].EncSignature, reader.data, blockReader)
		if err != nil {
			return err
		}

		reader.nextRevision = i + 1
	}

	return nil
}

func (protonDrive *ProtonDrive) DownloadFileByID(ctx context.Context, linkID string, offset int64) (io.ReadCloser, int64, *FileSystemAttrs, error) {
	return protonDrive.DownloadFileRevisionByID(ctx, linkID, "", offset)
}

// DownloadFileRevisionByID rejects a changed active revision before downloading.
// An empty revisionID selects the current active revision.
func (protonDrive *ProtonDrive) DownloadFileRevisionByID(ctx context.Context, linkID, revisionID string, offset int64) (io.ReadCloser, int64, *FileSystemAttrs, error) {
	/* It's like event system, we need to get the latest information before creating the move request! */
	protonDrive.removeLinkIDFromCache(linkID, false)

	link, err := protonDrive.getLink(ctx, linkID)
	if err != nil {
		return nil, 0, nil, err
	}

	return protonDrive.downloadFile(ctx, link, revisionID, offset)
}

func (protonDrive *ProtonDrive) DownloadFile(ctx context.Context, link *proton.Link, offset int64) (io.ReadCloser, int64, *FileSystemAttrs, error) {
	return protonDrive.downloadFile(ctx, link, "", offset)
}

func (protonDrive *ProtonDrive) downloadFile(ctx context.Context, link *proton.Link, revisionID string, offset int64) (io.ReadCloser, int64, *FileSystemAttrs, error) {
	if link.Type != proton.LinkTypeFile {
		return nil, 0, nil, ErrLinkTypeMustToBeFileType
	}

	parentNodeKR, err := protonDrive.getLinkKRByID(ctx, link.ParentLinkID)
	if err != nil {
		return nil, 0, nil, err
	}

	signatureVerificationKR, err := protonDrive.getSignatureVerificationKeyring([]string{link.SignatureEmail})
	if err != nil {
		return nil, 0, nil, err
	}
	nodeKR, err := link.GetKeyRing(parentNodeKR, signatureVerificationKR)
	if err != nil {
		return nil, 0, nil, err
	}

	sessionKey, err := link.GetSessionKey(nodeKR)
	if err != nil {
		return nil, 0, nil, err
	}

	revision, fileSystemAttrs, err := protonDrive.getActiveRevisionWithAttrs(ctx, link, revisionID)
	if err != nil {
		return nil, 0, nil, err
	}

	readerCtx, cancel := context.WithCancel(ctx)
	reader := &FileDownloadReader{
		protonDrive: protonDrive,
		ctx:         readerCtx,
		cancel:      cancel,

		link:         link,
		data:         bytes.NewBuffer(nil),
		nodeKR:       nodeKR,
		sessionKey:   sessionKey,
		revision:     revision,
		nextRevision: 0,

		isEOF: false,
	}

	if offset > 0 && (fileSystemAttrs == nil || fileSystemAttrs.BlockSizes == nil) {
		protonDrive.Config.GetLogger().Warnf("Performing inefficient seek as metadata of encrypted file is missing")
	}
	if err := reader.skipOffset(offset, fileSystemAttrs); err != nil {
		_ = reader.Close()
		return nil, 0, nil, err
	}
	return reader, link.Size, fileSystemAttrs, nil
}


// skipOffset advances to the requested plaintext byte even for legacy revisions
// without signed attributes. Otherwise a ranged download could return byte zero.
func (reader *FileDownloadReader) skipOffset(offset int64, attrs *FileSystemAttrs) error {
	if offset < 0 {
		return ErrSeekOffsetAfterSkippingBlocks
	}
	if offset == 0 {
		return nil
	}
	if attrs != nil && attrs.BlockSizes != nil && reader.revision != nil && len(attrs.BlockSizes) == len(reader.revision.Blocks) {
		// Skip complete encrypted blocks without downloading their contents.
		// Reject malformed sizes instead of selecting an incorrect block.
		for _, size := range attrs.BlockSizes {
			if size <= 0 {
				return ErrSeekOffsetAfterSkippingBlocks
			}
		}
		totalBytes := int64(0)
		for i := 0; i < len(attrs.BlockSizes); i++ {
			prevTotalBytes := totalBytes
			if attrs.BlockSizes[i] > int64(^uint64(0)>>1)-totalBytes {
				return ErrSeekOffsetAfterSkippingBlocks
			}
			totalBytes += attrs.BlockSizes[i]
			if offset <= totalBytes {
				offset -= prevTotalBytes
				reader.nextRevision = i
				break
			}
		}
		if offset > totalBytes {
			return ErrSeekOffsetAfterSkippingBlocks
		}
	}
	// Without block sizes, discard from the start rather than silently
	// ignoring offset. Partial reads and resumed transfers must remain correct.
	n, err := io.CopyN(io.Discard, reader, offset)
	if err != nil {
		return err
	}
	if n != offset {
		return ErrSeekOffsetAfterSkippingBlocks
	}
	return nil
}
