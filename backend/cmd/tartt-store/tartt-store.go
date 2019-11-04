// vim: sw=8

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/docopt/docopt-go"
	"github.com/nogproject/nog/backend/pkg/mulog"
)

// `xVersion` and `xBuild` are injected by the `Makefile`.
var (
	xVersion string
	xBuild   string
	version  = fmt.Sprintf("tartt-store-%s+%s", xVersion, xBuild)
)

// `qqBackticks()` translates double single quote to backtick.
func qqBackticks(s string) string {
	return strings.Replace(s, "''", "`", -1)
}

var usage = qqBackticks(strings.TrimSpace(`
Usage:
  tartt-store save [--datadir=<dir>] --split-zstd-gpg-split [--cipher-algo=<cipher>] --secret-fd=<n> [<basename>]
  tartt-store save [--datadir=<dir>] --gpg [--cipher-algo=<cipher>] --secret-fd=<n> [<basename>]
  tartt-store save [--datadir=<dir>] --direct [<basename>]
  tartt-store save [--datadir=<dir>] --split-gzip-split [<basename>]
  tartt-store save [--datadir=<dir>] --split-zstd-split [<basename>]
  tartt-store load [--datadir=<dir>] [--secret-stdin] [<basename>]

Options:
  --direct            Store stdin as a single uncompressed file.
  --gpg               Store stdin as a single gpg-encrypted file.
  --split-gzip-split  Split stdin into chunks that are compressed with gzip and
                      combined into a container tar that is then split again
                      into pieces that are stored.
  --split-zstd-split  Like --split-gzip-split but with zstd.
  --split-zstd-gpg-split  Like --split-gzip-split but with zstd and gpg.
  --cipher-algo=<cipher>  [default: AES]
                      Passed to ''gpg --cipher-algo'' when using encryption.
                      Supported ciphers: AES, AES192, AES256.
  --secret-stdin      Read the plaintext secret from stdin.
  --secret-fd=<n>     Read the plaintext secret from file descriptor ''<n>''.
  --datadir=<dir>     Save data to a different directory.  The manifest is
                      always stored in the current directory.

The default ''<basename>'' is ''data.tar''.  All examples below are for the
default basename.

''tartt-store save'' saves stdin into the current directory in the specified
format.  The default is ''--split-zstd-split''.

''tartt-store load'' auto-detects the storage format and writes the original
data to stdout.

Assuming the original data is a tar stream, as created by ''tartt'', its
content can be listed in a ''full/'' or ''patch/'' directory for all storage
formats as follows:

    tartt-store load | tar -tvf-

If it has been stored with ''--gpg'' and a plaintext secret:

    cat data.tar.gpg \
    | gpg2 --batch --decrypt \
        --passphrase-file secret \
    | tar -tvf-

If it has been stored with ''--gpg'' and an encrypted secret:

    cat data.tar.gpg \
    | gpg2 --batch --decrypt \
        --passphrase-file <(gpg2 --decrypt -o- secret.asc) \
    | tar -tvf-

If it has been stored with ''--split-zstd-gpg-split'' and a plaintext secret:

    cat data.tar.zst.gpg.tar.* | tar -xOf- \
    | gpg2 --batch --decrypt --allow-multiple-messages \
        --passphrase-file secret \
    | unzstd \
    | tar -tvf-

If it has been stored with ''--split-zstd-gpg-split'' and an encrypted secret:

    cat data.tar.zst.gpg.tar.* | tar -xOf- \
    | gpg2 --batch --decrypt --allow-multiple-messages \
        --passphrase-file <(gpg2 --decrypt -o- secret.asc) \
    | unzstd \
    | tar -tvf-

If it has been stored with ''--split-zstd-split'':

    cat data.tar.zst.tar.* | tar -xOf- | unzstd | tar -tvf-

If it has been stored with ''--split-gzip-split'':

    cat data.tar.gz.tar.* | tar -xOf- | gunzip | tar -tvf-

To authenticate data that has been signed with ''tartt sign'':

    gpg2 --verify manifest.shasums{.asc,}
    grep ^sha256: manifest.shasums | cut -d : -f 2 | sha256sum -c
    grep ^sha512: manifest.shasums | cut -d : -f 2 | sha512sum -c

`))

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
	Errorw(msg string, kv ...interface{})
	Fatalw(msg string, kv ...interface{})
}

type Cipher string

var lg Logger = mulog.Printer{}

func main() {
	args := argparse()

	switch {
	case args["save"].(bool):
		cmdSave(args)
	case args["load"].(bool):
		cmdLoad(args)
	default:
		panic("unhandled args")
	}
}

func argparse() map[string]interface{} {
	const autoHelp = true
	const noOptionFirst = false
	args, err := docopt.Parse(
		usage, nil, autoHelp, version, noOptionFirst,
	)
	if err != nil {
		lg.Fatalw("docopt failed.", "err", err)
	}

	if _, ok := args["<basename>"].(string); !ok {
		args["<basename>"] = "data.tar"
	}

	if arg, ok := args["--secret-fd"].(string); ok {
		v, err := strconv.ParseUint(arg, 10, 32)
		if err != nil {
			lg.Fatalw("Invalid --secret-fd.", "err", err)
		}
		args["--secret-fd"] = uintptr(v)
	}

	arg := args["--cipher-algo"].(string)
	switch arg {
	case "AES", "AES192", "AES256":
		args["--cipher-algo"] = Cipher(arg)
	default:
		lg.Fatalw("Invalid --cipher-algo.")
	}

	return args
}

func cmdSave(args map[string]interface{}) {
	basename := args["<basename>"].(string)

	var datadir string
	if a, ok := args["--datadir"].(string); ok {
		datadir = a
	}

	switch {
	case args["--split-zstd-gpg-split"].(bool):
		secret := mustReadSecret(args["--secret-fd"].(uintptr))
		cipher := args["--cipher-algo"].(Cipher)
		saveSplitZstdGPGSplit(datadir, basename, secret, cipher)
	case args["--gpg"].(bool):
		secret := mustReadSecret(args["--secret-fd"].(uintptr))
		cipher := args["--cipher-algo"].(Cipher)
		saveGPG(datadir, basename, secret, cipher)
	case args["--split-zstd-split"].(bool):
		saveSplitZstdSplit(datadir, basename)
	case args["--split-gzip-split"].(bool):
		saveSplitGzipSplit(datadir, basename)
	case args["--direct"].(bool):
		saveDirect(datadir, basename)
	default:
		panic("args logic error")
	}
}

func cmdLoad(args map[string]interface{}) {
	basename := args["<basename>"].(string)

	var datadir string
	if a, ok := args["--datadir"].(string); ok {
		datadir = a
	}

	var secret string
	if args["--secret-stdin"].(bool) {
		in, err := ioutil.ReadAll(os.Stdin)
		mustLoadSecret(err)
		secret = string(bytes.TrimSpace(in))
	}

	manifest := mustLoadManifestFile()

	switch {
	case isSplitZstdGPGSplit(manifest, basename):
		loadSplitZstdGPGSplit(manifest, datadir, basename, secret)
	case isGPG(manifest, basename):
		loadGPG(datadir, basename, secret)
	case isSplitZstdSplit(manifest, basename):
		loadSplitZstdSplit(manifest, datadir, basename)
	case isSplitGzipSplit(manifest, basename):
		loadSplitGzipSplit(manifest, datadir, basename)
	case isDirect(manifest, basename):
		loadDirect(datadir, basename)
	default:
		lg.Fatalw("Failed to determine archive storage format.")
	}
}

func mustReadSecret(fd uintptr) string {
	fp := os.NewFile(fd, "secret")
	if fp == nil {
		lg.Fatalw("Failed to read --secret-fd.")
	}
	defer fp.Close()
	in, err := ioutil.ReadAll(fp)
	mustLoadSecret(err)
	return string(bytes.TrimSpace(in))
}

func mustLoadManifestFile() *Manifest {
	fp, err := os.Open("manifest.shasums")
	mustLoadManifest(err)
	manifest, err := readManifest(fp)
	mustLoadManifest(err)
	mustLoadManifest(fp.Close())
	return manifest
}
