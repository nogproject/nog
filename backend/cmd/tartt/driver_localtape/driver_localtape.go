package driver_localtape

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nogproject/nog/backend/cmd/tartt/drivers"
	"github.com/nogproject/nog/backend/pkg/execx"
)

var ErrConfigMissingTardir = errors.New("missing config `localtape.tardir`")
var ErrTardirNotDir = errors.New("`localtape.tardir` is not a directory")
var ErrArchivExists = errors.New("the archive already exists")

type Logger interface {
	Infow(msg string, kv ...interface{})
}

type Driver struct {
	lg     Logger
	tardir string
}

type ArchiveTx struct {
	lg                       Logger
	final, inprogress, local string
	hasReadme                bool
}

func New(lg Logger, name string, cfgYml []byte) (*Driver, error) {
	cfg, err := parseConfig(cfgYml)
	if err != nil {
		return nil, err
	}

	tardir := cfg.findTardir(name)
	if tardir == "" {
		return nil, ErrConfigMissingTardir
	}
	if !isDir(tardir) {
		return nil, ErrTardirNotDir
	}

	return &Driver{lg: lg, tardir: tardir}, nil
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
	final := filepath.Join(d.tardir, dst)
	inprogress := fmt.Sprintf("%s.inprogress", final)

	if exists(final) {
		return nil, ErrArchivExists
	}

	if err := os.MkdirAll(inprogress, 0777); err != nil {
		return nil, err
	}

	return &ArchiveTx{
		lg:         d.lg,
		final:      final,
		inprogress: inprogress,
		local:      tmp,
	}, nil
}

func (d *Driver) RemoveAll(prefix string) error {
	abspath := filepath.Join(d.tardir, prefix)
	d.lg.Infow("Removed localtape files.", "prefix", abspath)
	return os.RemoveAll(abspath)
}

func (tx *ArchiveTx) Commit() error {
	if err := cp(
		filepath.Join(tx.local, "manifest.shasums"), tx.inprogress,
	); err != nil {
		return err
	}

	if tx.hasReadme {
		if err := cp(
			filepath.Join(tx.local, "README.md"), tx.inprogress,
		); err != nil {
			return err
		}
	}

	if err := os.Rename(tx.inprogress, tx.final); err != nil {
		return err
	}

	tx.lg.Infow("Completed localtape archive.", "dest", tx.final)
	return nil
}

func (tx *ArchiveTx) Abort() error {
	return os.RemoveAll(tx.inprogress)
}

func (tx *ArchiveTx) SaveProgram(tarttStore string) string {
	return tarttStore
}

func (tx *ArchiveTx) SaveArgs(args []string) []string {
	if isSaveReadme(args) {
		tx.hasReadme = true
	}
	if isSaveTar(args) {
		return append([]string{
			"save",
			fmt.Sprintf("--datadir=%s", tx.inprogress),
		}, args[1:]...)
	}
	return args
}

func (d *Driver) LoadProgram(tarttStore string) string {
	return tarttStore
}

func (d *Driver) LoadArgs(arRel string, args []string) []string {
	if isLoadTar(args) {
		arAbs := filepath.Join(d.tardir, arRel)
		return append([]string{
			"load",
			fmt.Sprintf("--datadir=%s", arAbs),
		}, args[1:]...)
	}
	return args
}

func isSaveReadme(args []string) bool {
	if len(args) < 1 {
		return false
	}
	return args[len(args)-1] == "README.md"
}

func isSaveTar(args []string) bool {
	if len(args) < 1 {
		return false
	}
	return strings.HasSuffix(args[len(args)-1], ".tar")
}

func isLoadTar(args []string) bool {
	return isSaveTar(args)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDir(path string) bool {
	inf, err := os.Stat(path)
	if err != nil {
		return false
	}
	return inf.IsDir()
}

func cp(src, dst string) error {
	cmd := exec.Command(cpTool.Path, src, dst)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

var cpTool = execx.MustLookTool(execx.ToolSpec{
	Program:   "cp",
	CheckArgs: []string{"--version"},
	CheckText: "cp (GNU coreutils)",
})
