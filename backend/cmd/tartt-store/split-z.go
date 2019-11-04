package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/DataDog/zstd"
)

// The total memory use is `O(maxChunkSize * maxNConcurrent)`, with a constant
// that depends on the per-chunk processing buffers.
const (
	MiB = 1024 * 1024
	GiB = 1024 * MiB

	maxChunkSize = 128 * MiB
	maxPieceSize = 256 * GiB

	maxNConcurrent = 12
)

func nConcurrentFromNumCPU() int {
	n := runtime.NumCPU()
	if n > 6 {
		n /= 2
	}
	if n > maxNConcurrent {
		n = maxNConcurrent
	}
	return n
}

func isSplitGzipSplit(mf *Manifest, basename string) bool {
	return mf.HasFile(fmt.Sprintf("%s.gz.tar.000", basename))
}

func isSplitZstdSplit(mf *Manifest, basename string) bool {
	return mf.HasFile(fmt.Sprintf("%s.zst.tar.000", basename))
}

type chunkTask struct {
	in  []byte
	out chan<- []byte
}

// `saveSplitGzipSplit()` splits the input into chunks of size `maxChunkSize`
// that are compressed with gzip and written to a single tar stream that
// contains the compressed chunks as files `0, 1, 2, ...`.  The tar stream is
// then split into pieces of size `maxPieceSize` and saved to disk, together
// with SHA manifest files.
//
// Channel `chunks` contains uncompressed `in` chunks together with a result
// write channel `out` for the compressed chunk.  The `out` read channels are
// sent to `results` in the original order.
//
// `readChunks()` splits the input and sends it to `chunks`.  The completion
// channels are sent to `results`.
//
// `gzipChunks()` reads from `chunks` and sends compressed chunks to the `out`
// completion channels.  `nConcurrent` goroutines compress in parallel.
//
// `tarChunks()` writes compressed chunks to the tar stream in the original
// order.  `splitSave()` splits the tar stream into pieces and writes them to
// disk.
func saveSplitGzipSplit(datadir, basename string) {
	nConcurrent := nConcurrentFromNumCPU()
	lg.Infow("Determined number of parallel gzip tasks.", "n", nConcurrent)

	chunks := make(chan chunkTask)
	results := make(chan (<-chan []byte), nConcurrent)
	tarR, tarW := io.Pipe()

	go readChunks(results, chunks, os.Stdin)
	for i := 0; i < nConcurrent; i++ {
		go gzipChunks(chunks)
	}
	go tarChunks(tarW, results)
	splitSave(datadir, basename, tarR, "gz")
}

func saveSplitZstdSplit(datadir, basename string) {
	nConcurrent := nConcurrentFromNumCPU()
	lg.Infow("Determined number of parallel zstd tasks.", "n", nConcurrent)

	chunks := make(chan chunkTask)
	results := make(chan (<-chan []byte), nConcurrent)
	tarR, tarW := io.Pipe()

	go readChunks(results, chunks, os.Stdin)
	for i := 0; i < nConcurrent; i++ {
		go zstdChunks(chunks)
	}
	go tarChunks(tarW, results)
	splitSave(datadir, basename, tarR, "zst")
}

func readChunks(
	results chan<- <-chan []byte,
	chunks chan<- chunkTask,
	r io.Reader,
) {
	for {
		chunkR := io.LimitReader(r, maxChunkSize)
		in, err := ioutil.ReadAll(chunkR)
		mustReceive(err)
		if len(in) == 0 {
			break
		}
		// Queue `in` for processing, awaiting the result on `out`.
		out := make(chan []byte)
		chunks <- chunkTask{in, out}
		results <- out
	}
	// Tell `xChunks()` that it's done.
	close(chunks)
	close(results)
}

func gzipChunks(chunks <-chan chunkTask) {
	for {
		chunk, ok := <-chunks
		if !ok {
			return
		}

		var gzBuf bytes.Buffer
		zw := gzip.NewWriter(&gzBuf)
		_, err := zw.Write(chunk.in)
		mustGzip(err)
		mustGzip(zw.Close())
		chunk.out <- gzBuf.Bytes()
	}
}

func zstdChunks(chunks <-chan chunkTask) {
	for {
		chunk, ok := <-chunks
		if !ok {
			return
		}

		var zstBuf bytes.Buffer
		zw := zstd.NewWriter(&zstBuf)
		_, err := zw.Write(chunk.in)
		mustZstd(err)
		mustZstd(zw.Close())
		chunk.out <- zstBuf.Bytes()
	}
}

func tarChunks(
	w io.WriteCloser,
	results <-chan <-chan []byte,
) {
	tw := tar.NewWriter(w)
	for i := 0; ; i++ {
		res, ok := <-results
		if !ok {
			break
		}
		out := <-res
		mustTar(tw.WriteHeader(&tar.Header{
			Name:    fmt.Sprintf("%d", i),
			Mode:    0400,
			Size:    int64(len(out)),
			ModTime: time.Now().UTC(),
		}))
		_, err := tw.Write(out)
		mustTar(err)
	}
	mustTar(tw.Close())
	mustTar(w.Close())
}

func splitSave(datadir, basename string, r io.Reader, zext string) {
	mf, err := os.OpenFile(
		"manifest.shasums", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644,
	)
	mustManifest(err)

	for i := 0; ; i++ {
		if i > 999 {
			err := errors.New("too many data pieces")
			mustSave(err)
		}
		path := fmt.Sprintf("%s.%s.tar.%03d", basename, zext, i)
		if !splitSaveOne(datadir, path, mf, r) {
			break
		}
	}

	mustManifest(mf.Sync())
	mustManifest(mf.Close())
}

func splitSaveOne(datadir, path string, mf io.Writer, r io.Reader) bool {
	fullPath := filepath.Join(datadir, path)

	// Copy a piece to the data file and concurrently compute SHAs.
	pieceR := io.LimitReader(r, maxPieceSize)

	fp, err := os.Create(fullPath)
	mustSave(err)

	sha256R, sha256W := io.Pipe()
	sha256C := make(chan string)
	go sha256sum(sha256C, sha256R) // Closes sha256R when done.

	sha512R, sha512W := io.Pipe()
	sha512C := make(chan string)
	go sha512sum(sha512C, sha512R) // Closes sha512R when done.

	n, err := io.Copy(io.MultiWriter(fp, sha256W, sha512W), pieceR)
	mustSave(err)
	mustSave(fp.Sync())
	mustSave(fp.Close())

	mustManifest(sha256W.Close())
	sha256hex := <-sha256C

	mustManifest(sha512W.Close())
	sha512hex := <-sha512C

	// The final piece might be empty.  If so, remove it and tell
	// `splitSave()` to stop.
	if n == 0 {
		mustSave(os.Remove(fullPath))
		return false
	}

	_, err = fmt.Fprintf(mf, "size:%d  %s\n", n, path)
	mustManifest(err)
	_, err = fmt.Fprintf(mf, "sha256:%s  %s\n", sha256hex, path)
	mustManifest(err)
	_, err = fmt.Fprintf(mf, "sha512:%s  %s\n", sha512hex, path)
	mustManifest(err)

	// Tell `splitSave()` to continue if this was a complete piece.
	return n == maxPieceSize
}

func loadSplitGzipSplit(mf *Manifest, datadir, basename string) {
	loadSplitXSplit(mf, datadir, basename, "gz", gunzip, "gunzip")
}

func loadSplitZstdSplit(mf *Manifest, datadir, basename string) {
	loadSplitXSplit(mf, datadir, basename, "zst", unzstd, "unzstd")
}

// `loadSplitXSplitSerial()` is a serial variant of the parallel
// `loadSplitXSplit()`.  The serial variant may be useful as a reference during
// development.  The parallel variant is used in production.
func loadSplitXSplitSerial(
	mf *Manifest, datadir, basename, ext string,
	unx func(w io.Writer, zR io.Reader),
	unxName string,
) {
	tarR, tarW := io.Pipe()
	go loadJoin(tarW, mf, datadir, basename, ext)
	untarChunksCat(os.Stdout, tarR, unx)
	mustSend(os.Stdout.Close())
}

func loadSplitXSplit(
	mf *Manifest, datadir, basename, ext string,
	unx func(w io.Writer, zR io.Reader),
	unxName string,
) {
	nConcurrent := nConcurrentFromNumCPU()
	lg.Infow(
		fmt.Sprintf(
			"Determined number of parallel %s tasks.", unxName,
		),
		"n", nConcurrent,
	)

	tarR, tarW := io.Pipe()
	chunks := make(chan chunkTask)
	results := make(chan (<-chan []byte), nConcurrent)

	go loadJoin(tarW, mf, datadir, basename, ext)
	go untarChunkTasks(results, chunks, tarR)
	for i := 0; i < nConcurrent; i++ {
		go unxChunks(chunks, unx)
	}
	catResults(os.Stdout, results)
	mustSend(os.Stdout.Close())
}

func loadJoin(w io.WriteCloser, mf *Manifest, datadir, basename, zext string) {
	files, err := mf.Glob(
		fmt.Sprintf("%s.%s.tar.[0-9][0-9][0-9]", basename, zext),
	)
	mustLoad(err)
	if len(files) < 1 {
		err := fmt.Errorf("missing %s.%s.tar.*", basename, zext)
		mustLoad(err)
	}
	for _, file := range files {
		fp, err := os.Open(filepath.Join(datadir, file))
		mustLoad(err)
		_, err = io.Copy(w, fp)
		mustLoad(err)
		mustLoad(fp.Close())
	}
	mustLoad(w.Close())
}

func unxChunks(
	chunks <-chan chunkTask,
	unx func(w io.Writer, zR io.Reader),
) {
	for {
		chunk, ok := <-chunks
		if !ok {
			return
		}
		var out bytes.Buffer
		unx(&out, bytes.NewReader(chunk.in))
		chunk.out <- out.Bytes()
	}
}

func catResults(
	w io.Writer,
	results <-chan <-chan []byte,
) {
	for {
		res, ok := <-results
		if !ok {
			return
		}
		out := <-res
		_, err := w.Write(out)
		mustSend(err)
	}
}

// `untarChunksCat()` reads chunks from the tar stream `r`, processes the
// chunks with `unz()`, and concatenates the result to `w`.
func untarChunksCat(
	w io.Writer, r io.Reader,
	unz func(w io.Writer, zR io.Reader),
) {
	tr := tar.NewReader(r)
	for {
		_, err := tr.Next()
		switch {
		case err == io.EOF:
			return // End of archive.
		case err != nil:
			mustUntar(err)
		}

		zR, zW := io.Pipe()
		go func() {
			_, err := io.Copy(zW, tr)
			mustUntar(err)
			mustUntar(zW.Close())
		}()
		unz(w, zR)
	}
}

// `untarChunksTasks()` reads chunks from the tar stream `r` into `in` byte
// slices.  It sends the `in` byte slices together with a result write channel
// `out` for the processed data to channel `chunks`.  It sends the `out` read
// channels to `results` in the original order, so that a consumer can receive
// the processed data in the original order.
func untarChunkTasks(
	results chan<- <-chan []byte,
	chunks chan<- chunkTask,
	r io.Reader,
) {
	tr := tar.NewReader(r)
	for {
		_, err := tr.Next()
		switch {
		case err == io.EOF:
			// End of archive.  Close channels to tell consumers to
			// quit.
			close(results)
			close(chunks)
			return
		case err != nil:
			mustUntar(err)
		}

		// Queue `in` for processing, awaiting the result on `out`.
		var in bytes.Buffer
		_, err = io.Copy(&in, tr)
		mustUntar(err)
		out := make(chan []byte)
		chunks <- chunkTask{in.Bytes(), out}
		results <- out
	}
}

func unzstd(w io.Writer, zR io.Reader) {
	zr := zstd.NewReader(zR)
	_, err := io.Copy(w, zr)
	mustUnzstd(err)
	mustUnzstd(zr.Close())
}

func gunzip(w io.Writer, zR io.Reader) {
	zr, err := gzip.NewReader(zR)
	mustGunzip(err)
	_, err = io.Copy(w, zr)
	mustGunzip(err)
	mustGunzip(zr.Close())
}
