package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/DataDog/zstd"
	"github.com/nogproject/nog/backend/pkg/execx"
)

func isSplitZstdGPGSplit(mf *Manifest, basename string) bool {
	return mf.HasFile(fmt.Sprintf("%s.zst.gpg.tar.000", basename))
}

// `saveSplitZstdGPGSplit()` is like `saveSplitGzipSplit()`, but with zstd
// followed by gpg AES256 encryption.
func saveSplitZstdGPGSplit(datadir, basename, secret string, cipher Cipher) {
	nConcurrent := nConcurrentFromNumCPU()
	lg.Infow(
		"Determined number of parallel zstd|gpg tasks.",
		"n", nConcurrent,
	)

	chunks := make(chan chunkTask)
	results := make(chan (<-chan []byte), nConcurrent)
	tarR, tarW := io.Pipe()

	go readChunks(results, chunks, os.Stdin)
	for i := 0; i < nConcurrent; i++ {
		go zstdGPGChunks(chunks, secret, cipher)
	}
	go tarChunks(tarW, results)
	splitSave(datadir, basename, tarR, "zst.gpg")
}

func zstdGPGChunks(chunks <-chan chunkTask, secret string, cipher Cipher) {
	for {
		chunk, ok := <-chunks
		if !ok {
			return
		}
		zstdGPGChunk(chunk, secret, cipher)
	}
}

func zstdGPGChunk(chunk chunkTask, secret string, cipher Cipher) {
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
		"--compress-algo", "Uncompressed",
	}
	gpgCmd := exec.Command(gpg2Tool.Path, args...)
	gpgCmd.Stderr = os.Stderr

	// Write secret to gpg2 via fd 3.
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

	// Write Zstd plaintext chunk to gpg2 stdin.
	gpgStdin, err := gpgCmd.StdinPipe()
	mustGpg(err)
	zw := zstd.NewWriter(gpgStdin)
	zDone := make(chan error)
	go func() {
		_, err := zw.Write(chunk.in)
		if err2 := zw.Close(); err == nil {
			err = err2
		}
		if err2 := gpgStdin.Close(); err == nil {
			err = err2
		}
		zDone <- err
	}()

	// Run gpg2, reading the ciphertext from stdout.
	var gpgBuf bytes.Buffer
	gpgCmd.Stdout = &gpgBuf
	mustGpg(gpgCmd.Run())
	mustGpg(<-passDone)
	mustGpg(<-zDone)

	chunk.out <- gpgBuf.Bytes()
}

func loadSplitZstdGPGSplit(mf *Manifest, datadir, basename, secret string) {
	if secret == "" {
		secret = loadSecret()
	}
	unz := gpgUnzstdFunc(secret)
	loadSplitXSplit(mf, datadir, basename, "zst.gpg", unz, "gpg|unzstd")
}

// Capture `secret` in closure.
func gpgUnzstdFunc(secret string) func(w io.Writer, gpgR io.Reader) {
	return func(w io.Writer, gpgR io.Reader) {
		args := []string{
			"--batch",
			// See `zstdGPGOneChunk()` for details.
			"--passphrase-fd", "3",
			"--decrypt",
		}
		gpgCmd := exec.Command(gpg2Tool.Path, args...)
		gpgCmd.Stderr = os.Stderr

		// Write secret to gpg2 via fd 3.
		passR, passW, err := os.Pipe()
		mustGpg(err)
		defer passR.Close()
		// `passW.Close()` in goroutine after write.
		gpgCmd.ExtraFiles = []*os.File{
			passR, // fd 3
		}
		passDone := make(chan error)
		go func() {
			_, err := io.WriteString(passW, secret)
			if err2 := passW.Close(); err == nil {
				err = err2
			}
			passDone <- err
		}()

		// Write ciphertext to gpg2 stdin.
		gpgCmd.Stdin = gpgR

		// Read cleartext from gpg2 stdout and uncompress.
		gpgStdout, err := gpgCmd.StdoutPipe()
		mustGpg(err)
		mustGpg(gpgCmd.Start())
		zr := zstd.NewReader(gpgStdout)
		_, err = io.Copy(w, zr)
		mustUnzstd(err)
		mustUnzstd(zr.Close())
		mustGpg(gpgCmd.Wait())
		mustGpg(<-passDone)
	}
}

func loadSecret() string {
	if exists("secret.asc") {
		return loadEncryptedSecret("secret.asc")
	} else if exists("secret") {
		return loadPlaintextSecret("secret")
	}
	err := errors.New("found neither file `secret.asc` nor `secret`")
	mustLoadSecret(err)
	return ""
}

func loadEncryptedSecret(path string) string {
	gpgArgs := []string{
		"--batch",
		"--decrypt", path,
	}
	gpgCmd := exec.Command(gpg2Tool.Path, gpgArgs...)
	gpgCmd.Stderr = os.Stderr
	var secret bytes.Buffer
	gpgCmd.Stdout = &secret
	mustLoadSecret(gpgCmd.Run())
	return strings.TrimSpace(secret.String())
}

func loadPlaintextSecret(path string) string {
	secret, err := ioutil.ReadFile(path)
	mustLoadSecret(err)
	return strings.TrimSpace(string(secret))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var gpg2Tool = execx.MustLookTool(execx.ToolSpec{
	Program:   "gpg2",
	CheckArgs: []string{"--version"},
	CheckText: "gpg (GnuPG) 2.",
})
