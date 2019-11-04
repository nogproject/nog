package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func isDirect(mf *Manifest, basename string) bool {
	return mf.HasFile(basename)
}

func saveDirect(datadir, basename string) {
	path := basename
	fullPath := filepath.Join(datadir, path)

	fp, err := os.Create(fullPath)
	mustSave(err)

	sha256R, sha256W := io.Pipe()
	sha256Done := make(chan string)
	go sha256sum(sha256Done, sha256R) // Closes sha256R when done.

	sha512R, sha512W := io.Pipe()
	sha512Done := make(chan string)
	go sha512sum(sha512Done, sha512R) // Closes sha512R when done.

	n, err := io.Copy(io.MultiWriter(fp, sha256W, sha512W), os.Stdin)
	mustSave(err)
	mustSave(fp.Sync())
	mustSave(fp.Close())

	mustManifest(sha256W.Close())
	sha256hex := <-sha256Done

	mustManifest(sha512W.Close())
	sha512hex := <-sha512Done

	mf, err := os.OpenFile(
		"manifest.shasums", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644,
	)
	mustManifest(err)
	_, err = fmt.Fprintf(mf, "size:%d  %s\n", n, path)
	mustManifest(err)
	_, err = fmt.Fprintf(mf, "sha256:%s  %s\n", sha256hex, path)
	mustManifest(err)
	_, err = fmt.Fprintf(mf, "sha512:%s  %s\n", sha512hex, path)
	mustManifest(err)
	mustManifest(mf.Sync())
	mustManifest(mf.Close())
}

func loadDirect(datadir, basename string) {
	fp, err := os.Open(filepath.Join(datadir, basename))
	mustLoad(err)
	_, err = io.Copy(os.Stdout, fp)
	mustLoad(err)
	mustLoad(os.Stdout.Close())
}
