package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func isGPG(mf *Manifest, basename string) bool {
	return mf.HasFile(fmt.Sprintf("%s.gpg", basename))
}

func saveGPG(datadir, basename, secret string, cipher Cipher) {
	path := fmt.Sprintf("%s.gpg", basename)
	fullPath := filepath.Join(datadir, path)

	fp, err := os.Create(fullPath)
	mustSave(err)

	args := []string{
		"--batch",
		// Do not:
		//
		// ```
		// "--pinentry-mode", "loopback",
		// ````
		//
		// The GnuPG documentation states that `--pinentry-mode` is
		// required with `--passphrase-fd` since 2.1, see
		// <http://bit.ly/2v9P5wC>.  But in practice, it works without:
		//
		//  - gpg 2.1.11: error with --pinentry-mode, works without.
		//  - gpg 2.1.18: works with or without.
		//
		"--passphrase-fd", "3",
		"--symmetric",
		"--cipher-algo", string(cipher),
		"--compress-algo", "ZLIB",
	}
	gpgCmd := exec.Command(gpg2Tool.Path, args...)
	gpgCmd.Stdin = os.Stdin
	gpgStdout, err := gpgCmd.StdoutPipe()
	mustGpg(err)
	gpgCmd.Stderr = os.Stderr

	// Pass secret via fd 3.
	passR, passW, err := os.Pipe()
	mustGpg(err)
	defer passR.Close()
	// `passW.Close()` in goroutine after write.
	gpgCmd.ExtraFiles = []*os.File{
		passR, // `ExtraFiles` start with fd 3.
	}
	passDone := make(chan error)
	go func() {
		_, err := io.WriteString(passW, secret)
		if err2 := passW.Close(); err == nil {
			err = err2
		}
		passDone <- err
	}()

	mustGpg(gpgCmd.Start())

	sha256R, sha256W := io.Pipe()
	sha256Done := make(chan string)
	go sha256sum(sha256Done, sha256R) // Closes sha256R when done.

	sha512R, sha512W := io.Pipe()
	sha512Done := make(chan string)
	go sha512sum(sha512Done, sha512R) // Closes sha512R when done.

	n, err := io.Copy(io.MultiWriter(fp, sha256W, sha512W), gpgStdout)
	mustGpg(err)
	mustGpg(gpgCmd.Wait())
	mustGpg(<-passDone)
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

func loadGPG(datadir, basename, secret string) {
	if secret == "" {
		secret = loadSecret()
	}

	fullPath := filepath.Join(datadir, fmt.Sprintf("%s.gpg", basename))
	args := []string{
		"--batch",
		// See `zstdGPGOneChunk()` for details.
		"--passphrase-fd", "3",
		"--decrypt",
		"--output", "-",
		fullPath,
	}
	gpgCmd := exec.Command(gpg2Tool.Path, args...)
	gpgCmd.Stdin = nil
	gpgCmd.Stdout = os.Stdout
	gpgCmd.Stderr = os.Stderr

	// Pass secret via fd 3.
	passR, passW, err := os.Pipe()
	mustGpg(err)
	defer passR.Close()
	// passW.Close() below after write.
	gpgCmd.ExtraFiles = []*os.File{
		passR, // `ExtraFiles` start with fd 3.
	}

	mustGpg(gpgCmd.Start())
	_, err = io.WriteString(passW, secret)
	mustGpg(err)
	mustGpg(passW.Close())
	mustGpg(gpgCmd.Wait())
}
