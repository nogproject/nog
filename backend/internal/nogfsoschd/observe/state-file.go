package observe

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/nogproject/nog/backend/pkg/ulid"
)

type FileStateStore struct {
	dir string
}

func NewFileStateStore(dir string) *FileStateStore {
	return &FileStateStore{
		dir: dir,
	}
}

func (s *FileStateStore) LoadULID(name string) (ulid.I, error) {
	data, err := ioutil.ReadFile(
		filepath.Join(s.dir, fmt.Sprintf("%s.ulid", name)),
	)
	if os.IsNotExist(err) {
		return ulid.Nil, nil
	} else if err != nil {
		return ulid.Nil, err
	}

	id, err := ulid.Parse(string(bytes.TrimSpace(data)))
	if err != nil {
		return ulid.Nil, err
	}

	return id, nil
}

func (s *FileStateStore) SaveULID(name string, id ulid.I) error {
	base := fmt.Sprintf("%s.ulid", name)
	tmp, err := ioutil.TempFile(s.dir, fmt.Sprintf("%s.tmp.", base))
	if err != nil {
		return err
	}
	defer func() {
		if tmp != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()

	if _, err := io.WriteString(
		tmp, fmt.Sprintf("%s\n", id.String()),
	); err != nil {
		return err
	}
	// No `tmp.Flush()`.  It's not worth it.  The scheduled commands must
	// be able to handle restarts from any point anyway.
	if err := tmp.Close(); err != nil {
		return err
	}

	dst := filepath.Join(s.dir, base)
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return err
	}
	tmp = nil

	return nil
}
