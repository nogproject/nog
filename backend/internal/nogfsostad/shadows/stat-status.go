package shadows

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

type StatStatusFunc func(ps pb.PathStatus) error

var ErrInvalidGitFsoStatusOutput = errors.New(
	"invalid git-fso status output",
)

func (fs *Filesystem) StatStatus(
	ctx context.Context,
	shadowPath string,
	callback StatStatusFunc,
) error {
	if err := fs.checkShadowPath(shadowPath); err != nil {
		return err
	}

	args := []string{"status", "--stat", "-z"}
	cmd := exec.CommandContext(ctx, fs.tools.gitFso.Path, args...)
	cmd.Dir = shadowPath

	cmdOut, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	var cmdStderr bytes.Buffer
	cmd.Stderr = &cmdStderr

	err = cmd.Start()
	if err != nil {
		return err
	}

	send := func() error {
		scan := bufio.NewScanner(cmdOut)
		scan.Split(ScanZero)
		for scan.Scan() {
			line := scan.Text()
			if len(line) < 3 {
				return ErrInvalidGitFsoStatusOutput
			}
			stChar := line[0]
			path := line[2:]

			st := map[byte]pb.PathStatus_Status{
				'?': pb.PathStatus_PS_NEW,
				'M': pb.PathStatus_PS_MODIFIED,
				'D': pb.PathStatus_PS_DELETED,
			}[stChar]
			if st == pb.PathStatus_PS_UNSPECIFIED {
				return ErrInvalidGitFsoStatusOutput
			}

			if err := callback(pb.PathStatus{
				Path:   path,
				Status: st,
			}); err != nil {
				return err
			}
		}
		return scan.Err()
	}
	if err := send(); err != nil {
		_, _ = io.Copy(ioutil.Discard, cmdOut)
		_ = cmdOut.Close() // In case io.Copy stopped due to error.
		_ = cmd.Wait()
		return err
	}

	err = cmd.Wait()
	if err != nil {
		err = fmt.Errorf(
			"%s; git-fso stderr: %s",
			err, cmdStderr.String(),
		)
	}
	return err
}

// Base on `bufio.ScanLines()`.
func ScanZero(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, 0); i >= 0 {
		// We have a full zero-terminated line.
		return i + 1, data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}
