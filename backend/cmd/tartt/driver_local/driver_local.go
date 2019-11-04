package driver_local

import (
	"github.com/nogproject/nog/backend/cmd/tartt/drivers"
)

type Driver struct{}
type ArchiveTx struct{}

func New(name string, cfgYml []byte) (*Driver, error) {
	return &Driver{}, nil
}

func (d *Driver) Open() (drivers.StoreHandle, error) {
	return d, nil
}

func (d *Driver) Close() error {
	return nil
}

func (d *Driver) BeginArchive(
	dst, tmp string,
) (drivers.ArchiveTx, error) {
	return &ArchiveTx{}, nil
}

func (d *Driver) RemoveAll(prefix string) error {
	return nil
}

func (tx *ArchiveTx) Commit() error {
	return nil
}

func (tx *ArchiveTx) Abort() error {
	return nil
}

func (tx *ArchiveTx) SaveProgram(tarttStore string) string {
	return tarttStore
}

func (tx *ArchiveTx) SaveArgs(args []string) []string {
	return args
}

func (d *Driver) LoadProgram(tarttStore string) string {
	return tarttStore
}

func (d *Driver) LoadArgs(arRel string, args []string) []string {
	return args
}
