package main

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"io"
)

func sha256sum(out chan<- string, r io.ReadCloser) {
	h := sha256.New()
	_, err := io.Copy(h, r)
	mustManifest(err)
	_ = r.Close()
	out <- hex.EncodeToString(h.Sum(nil))
}

func sha512sum(out chan<- string, r io.ReadCloser) {
	h := sha512.New()
	_, err := io.Copy(h, r)
	mustManifest(err)
	_ = r.Close()
	out <- hex.EncodeToString(h.Sum(nil))
}
