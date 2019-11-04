package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	slashpath "path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/hcl"
	"github.com/nogproject/nog/backend/cmd/tartt/driver_local"
	"github.com/nogproject/nog/backend/cmd/tartt/driver_localtape"
	"github.com/nogproject/nog/backend/cmd/tartt/drivers"
	"github.com/nogproject/nog/backend/pkg/flock"
	yaml "gopkg.in/yaml.v2"
)

var ErrPathTooShort = errors.New("path too short")
var ErrFirstLevelDisabled = errors.New("first level must be enabled")
var ErrLastLevelDisabled = errors.New("last level must be enabled")

type RepoConfig struct {
	OriginDir string        `yaml:"originDir"`
	StoresDir string        `yaml:"storesDir"`
	Stores    []StoreConfig `yaml:"stores"`
}

type StoreConfig struct {
	Name   string        `yaml:"name"`
	Driver string        `yaml:"driver"`
	Levels []LevelConfig `yaml:"levels"`
}

type LevelConfig struct {
	Interval string `yaml:"interval"`
	Lifetime string `yaml:"lifetime"`
	// If a level is `Disabled`, no new tars will be added.  The first and
	// last level must always be enabled.
	Disabled bool `yaml:"disabled"`
}

type Repo struct {
	repoDir   string
	originDir string
	storesDir string
	// `stores` are partially initialized.  Use them only through
	// `Repo.OpenStore()`.
	stores []*Store
}

type Store struct {
	// Valid when partially initialized.
	Name     string
	storeDir string
	levels   []*Level
	driver   drivers.StoreDriver

	// Valid when fully initialized, as returned from `Repo.OpenStore()`.
	handle drivers.StoreHandle
	lock   *flock.Flock
}

type Level struct {
	interval LevelDuration
	lifetime LevelDuration
	disabled bool
}

func (r *Repo) OriginDir() string {
	return r.originDir
}

func (s *Store) IsRootLevel(level *Level) bool {
	return level == s.levels[0]
}

func (s *Store) Dir() string {
	return s.storeDir
}

func (s *Store) AbsPath(p string) string {
	return filepath.FromSlash(slashpath.Join(s.storeDir, p))
}

func (s *Store) ArchiveHandler() drivers.ArchiveHandler {
	return s.handle
}

func (s *Store) GcHandler() drivers.GcHandler {
	return s.handle
}

func (s *Store) UntarHandler() drivers.UntarHandler {
	return s.handle
}

func SplitStoreTspath(path string) (string, string, error) {
	st := strings.SplitN(path, "/", 2)
	if len(st) != 2 {
		return "", "", ErrPathTooShort
	}
	return st[0], st[1], nil
}

func (s *Store) TryLock(ctx context.Context) error {
	return s.lock.TryLock(ctx, 500*time.Millisecond)
}

func (s *Store) Unlock() error {
	return s.lock.Unlock()
}

func (lv *Level) Name() string {
	switch lv.interval.Unit {
	case DurationZero:
		return "s0"
	case DurationMinutes:
		return fmt.Sprintf("min%d", lv.interval.Value)
	case DurationHours:
		return fmt.Sprintf("h%d", lv.interval.Value)
	case DurationDays:
		return fmt.Sprintf("d%d", lv.interval.Value)
	case DurationMonths:
		return fmt.Sprintf("mo%d", lv.interval.Value)
	default:
		panic("invalid Level.interval")
	}
}

type DurationUnit int

const (
	DurationUnitUnspecified DurationUnit = iota
	DurationZero
	DurationMinutes
	DurationHours
	DurationDays
	DurationMonths
)

type LevelDuration struct {
	Value int
	Unit  DurationUnit
}

func (d LevelDuration) AddTime(t time.Time) time.Time {
	switch d.Unit {
	case DurationUnitUnspecified:
		panic("cannot add unspecified duration unit")
	case DurationZero:
		return t
	case DurationMinutes:
		return t.Add(time.Duration(d.Value) * time.Minute)
	case DurationHours:
		return t.Add(time.Duration(d.Value) * time.Hour)
	case DurationDays:
		return t.AddDate(0, 0, d.Value)
	case DurationMonths:
		return t.AddDate(0, d.Value, 0)
	default:
		panic("invalid LevelDuration")
	}
}

var durationRgxs = map[DurationUnit]*regexp.Regexp{
	DurationMinutes: regexp.MustCompile(`^([1-9][0-9]*) minutes?$`),
	DurationHours:   regexp.MustCompile(`^([1-9][0-9]*) hours?$`),
	DurationDays:    regexp.MustCompile(`^([1-9][0-9]*) days?$`),
	DurationMonths:  regexp.MustCompile(`^([1-9][0-9]*) months?$`),
}

func ParseDateTimeDuration(s string) (LevelDuration, error) {
	if s == "0" {
		return LevelDuration{Unit: DurationZero}, nil
	}

	for unit, rgx := range durationRgxs {
		m := rgx.FindStringSubmatch(s)
		if m != nil {
			v, _ := strconv.Atoi(m[1])
			return LevelDuration{Value: v, Unit: unit}, nil
		}
	}

	return LevelDuration{}, errors.New("failed to parse duration")
}

func OpenRepo(path string) (*Repo, error) {
	if !isDir(path) {
		err := fmt.Errorf("repo path `%s` is not a directory", path)
		return nil, err
	}
	repoDir, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	r := &Repo{repoDir: repoDir}

	cfgYmlFile := filepath.Join(path, "tarttconfig.yml")
	cfgHcl := filepath.Join(path, "tartt.config.hcl") // DEPRECATED
	var cfgYml []byte
	var cfg RepoConfig
	if exists(cfgYmlFile) {
		dat, err := ioutil.ReadFile(cfgYmlFile)
		if err != nil {
			return nil, err
		}
		cfgYml = dat
		if err := yaml.Unmarshal(cfgYml, &cfg); err != nil {
			return nil, err
		}
	} else if exists(cfgHcl) {
		if err := loadConfigHcl(cfgHcl, &cfg); err != nil {
			return nil, err
		}
		lg.Warnw(
			"DEPRECATED `tartt.config.hcl` config.  " +
				"Driver options are not supported.  " +
				"You should migrate to `tarttconfig.yml`.",
		)
	} else {
		return nil, errors.New("missing config file")
	}

	if !filepath.IsAbs(cfg.OriginDir) {
		return nil, errors.New("`originDir` must be an absolute path")
	}
	// Warn but do not fail if origin cannot be checked.  Accessing origin
	// may require privileges.  Opening the repo without privileges,
	// however, may be useful, even though some operations may fail later.
	if inf, err := os.Stat(cfg.OriginDir); err != nil {
		lg.Warnw("Could not confirm that origin is a directory.")
	} else if !inf.IsDir() {
		return nil, errors.New("origin is not a directory")
	}
	r.originDir = cfg.OriginDir

	var storesDir string
	switch {
	case cfg.StoresDir == "":
		storesDir = filepath.Join(repoDir, "stores")
	case filepath.IsAbs(cfg.StoresDir):
		storesDir = cfg.StoresDir
	default:
		storesDir = filepath.Join(repoDir, cfg.StoresDir)
	}
	if !isDir(storesDir) {
		return nil, fmt.Errorf("missing stores dir `%s`", storesDir)
	}
	r.storesDir = storesDir

	for _, c := range cfg.Stores {
		s, err := newStorePartialInit(storesDir, c, cfgYml)
		if err != nil {
			return nil, err
		}
		r.stores = append(r.stores, s)
	}
	if len(r.stores) != 1 {
		err := errors.New("require exactly one store")
		return nil, err
	}

	return r, nil
}

// Stores must be declared in HCL as
//
// ```
// stores "foo" {
//     name = "foo"
//     ...
// }
// ```
//
// See HCL `array "" { ... }` decoding issue
// <https://github.com/hashicorp/hcl/issues/164>,
// <https://github.com/hashicorp/hcl/pull/228>.
func loadConfigHcl(file string, cfg *RepoConfig) error {
	cfgHcl, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	return hcl.Unmarshal(cfgHcl, cfg)
}

func newStorePartialInit(
	storesDir string, cfg StoreConfig, cfgYml []byte,
) (*Store, error) {
	storeDir := filepath.Join(storesDir, cfg.Name)
	if !isDir(storeDir) {
		err := fmt.Errorf("missing store dir `%s`", storeDir)
		return nil, err
	}

	var driver drivers.StoreDriver
	switch cfg.Driver {
	case "intree":
		lg.Warnw("DEPRECATED driver `intree`; change it to `local`.")
		fallthrough
	case "local":
		d, err := driver_local.New(cfg.Name, cfgYml)
		if err != nil {
			return nil, err
		}
		driver = d
	case "localtape":
		d, err := driver_localtape.New(lg, cfg.Name, cfgYml)
		if err != nil {
			return nil, err
		}
		driver = d
	default:
		err := errors.New("invalid store driver")
		return nil, err
	}
	s := &Store{
		Name:     cfg.Name,
		storeDir: storeDir,
		driver:   driver,
	}

	if len(cfg.Levels) < 2 {
		return nil, errors.New("expected at least 2 levels")
	}
	for i, c := range cfg.Levels {
		lv, err := newLevel(c)
		if err != nil {
			err := fmt.Errorf(
				"failed to parse level %d: %s", i, err,
			)
			return nil, err
		}
		s.levels = append(s.levels, lv)
	}
	if s.levels[0].disabled {
		return nil, ErrFirstLevelDisabled
	}
	if s.levels[len(s.levels)-1].disabled {
		return nil, ErrLastLevelDisabled
	}
	if s.levels[len(s.levels)-1].interval.Unit != DurationZero {
		err := errors.New("highest level must have interval 0")
		return nil, err
	}

	return s, nil
}

func (r *Repo) StoreNames() []string {
	ns := make([]string, len(r.stores))
	for i, s := range r.stores {
		ns[i] = s.Name
	}
	return ns
}

func (r *Repo) DefaultStoreName() string {
	return r.stores[0].Name
}

func (r *Repo) OpenStore(name string) (*Store, error) {
	for _, s := range r.stores {
		if s.Name == name {
			return s.dupOpen()
		}
	}
	return nil, errors.New("unknown store")
}

func (s *Store) dupOpen() (*Store, error) {
	if s.lock != nil {
		return nil, errors.New("store has been opened before")
	}

	dup := *s

	handle, err := s.driver.Open()
	if err != nil {
		return nil, err
	}
	dup.handle = handle

	lock, err := flock.Open(dup.storeDir)
	if err != nil {
		return nil, err
	}
	dup.lock = lock

	return &dup, nil
}

func newLevel(cfg LevelConfig) (*Level, error) {
	interval, err := ParseDateTimeDuration(cfg.Interval)
	if err != nil {
		err := fmt.Errorf("failed to parse interval: %v", err)
		return nil, err
	}

	lifetime, err := ParseDateTimeDuration(cfg.Lifetime)
	if err != nil {
		err := fmt.Errorf("failed to parse lifetime: %v", err)
		return nil, err
	}

	if interval.Unit == DurationZero {
		// Any lifetime unit is valid.
	} else if interval.Unit != lifetime.Unit {
		err := errors.New("mismatching interval and lifetime units")
		return nil, err
	}

	return &Level{
		interval: interval,
		lifetime: lifetime,
		disabled: cfg.Disabled,
	}, nil
}

func (r *Repo) Close() error {
	return nil
}

func (s *Store) Close() error {
	s.lock.Close()
	return s.handle.Close()
}

func isDir(path string) bool {
	inf, err := os.Stat(path)
	if err != nil {
		return false
	}
	return inf.IsDir()
}
