package protondrive

import (
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"sync"
)

func normaliseSHA1(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil // Unavailable, never an assertion that two files are equal.
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 20 {
		return "", errors.New("invalid plaintext SHA1 metadata returned by Proton Drive")
	}
	return value, nil
}

type onceReadCloser struct {
	io.ReadCloser
	once sync.Once
	err  error
}

func (r *onceReadCloser) Close() error {
	r.once.Do(func() { r.err = r.ReadCloser.Close() })
	return r.err
}

func limitedDownload(reader io.ReadCloser, limit int64) io.ReadCloser {
	var content io.Reader = reader
	if limit >= 0 {
		content = io.LimitReader(reader, limit)
	}
	return &onceReadCloser{ReadCloser: &downloadRange{Reader: content, Closer: reader}}
}

type downloadRange struct {
	io.Reader
	io.Closer
}
