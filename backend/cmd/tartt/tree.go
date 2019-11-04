package main

import (
	"errors"
	"fmt"
	"os"
	slashpath "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nogproject/nog/backend/pkg/regexpx"
)

type Tree struct {
	Root *LevelTree
}

type TreeNode interface {
	Name() string
	Level() *Level
	MinTime() time.Time
	MaxTime() time.Time
}

type LevelTree struct {
	name  string
	level *Level
	Times []*TimeTree
}

type TimeTree struct {
	name      string
	Time      time.Time
	level     *Level
	TarType   TarType
	SubLevels map[string]*LevelTree
}

func (t *LevelTree) Name() string  { return t.name }
func (t *LevelTree) Level() *Level { return t.level }

func (t *TimeTree) Name() string  { return t.name }
func (t *TimeTree) Level() *Level { return t.level }

func (t *Tree) MinTime() time.Time {
	if t.Root == nil {
		return time.Time{}
	}
	return t.Root.MinTime()
}

func (t *Tree) MaxTime() time.Time {
	if t.Root == nil {
		return time.Time{}
	}
	return t.Root.MaxTime()
}

func (t *LevelTree) MinTime() time.Time {
	return t.Times[0].MinTime()
}

func (t *LevelTree) MaxTime() time.Time {
	return t.Times[len(t.Times)-1].MaxTime()
}

func (t *TimeTree) MinTime() time.Time {
	return t.Time
}

func (t *TimeTree) MaxTime() time.Time {
	max := t.Time
	for _, child := range t.SubLevels {
		cMax := child.MaxTime()
		if cMax.After(max) {
			max = cMax
		}
	}
	return max
}

func (t *Tree) Find(tspath string) TreeNode {
	if t.Root == nil {
		return nil
	}
	if tspath == "" {
		return nil
	}
	if tspath == "." {
		return t.Root
	}
	return t.Root.find(strings.Split(tspath, "/"))
}

func (t *LevelTree) find(tspath []string) TreeNode {
	if len(tspath) == 0 {
		return t
	}
	ts := tspath[0]
	for _, child := range t.Times {
		if child.Name() == ts {
			return child.find(tspath[1:])
		}
	}
	return nil
}

func (t *TimeTree) find(tspath []string) TreeNode {
	if len(tspath) == 0 {
		return t
	}
	lvName := tspath[0]
	if child, ok := t.SubLevels[lvName]; ok {
		return child.find(tspath[1:])
	}
	return nil
}

// `TarType` enumerates the tar types.  The `TarX` constants can be combined to
// a `TarTypeMask` to select multiple tar types, e.g:
//
//	tree, err := LsTreeSelect(TarTypeMask(TarFull | TarPatch))
//
type TarType uint
type TarTypeMask TarType

const (
	TarFull TarType = 1 << iota
	TarPatch
	TarFullInProgress
	TarFullError
	TarPatchInProgress
	TarPatchError

	TarUnspecified = 0

	TarTypesAll TarTypeMask = TarTypeMask(0 |
		TarFull | TarPatch |
		TarFullInProgress | TarFullError |
		TarPatchInProgress | TarPatchError,
	)
)

func TarTypeFromTsDir(dir string) TarType {
	switch {
	case isDir(filepath.Join(dir, "full")):
		return TarFull
	case isDir(filepath.Join(dir, "patch")):
		return TarPatch
	case isDir(filepath.Join(dir, "full.inprogress")):
		return TarFullInProgress
	case isDir(filepath.Join(dir, "full.error")):
		return TarFullError
	case isDir(filepath.Join(dir, "patch.inprogress")):
		return TarPatchInProgress
	case isDir(filepath.Join(dir, "patch.error")):
		return TarPatchError
	default:
		// Handle missing tar dir as unspecified.
		return TarUnspecified
	}
}

func (t TarType) Path() string {
	switch t {
	case TarFull:
		return "full"
	case TarPatch:
		return "patch"
	case TarFullInProgress:
		return "full.inprogress"
	case TarFullError:
		return "full.error"
	case TarPatchInProgress:
		return "patch.inprogress"
	case TarPatchError:
		return "patch.error"
	case TarUnspecified:
		panic("unspecified TarType")
	default:
		panic("invalid TarType")
	}
}

func (s *Store) LsTree() (*Tree, error) {
	return s.LsTreeSelect(TarTypeMask(TarFull | TarPatch))
}

func (s *Store) LsTreeSelect(sel TarTypeMask) (*Tree, error) {
	root, err := lsLevelDir(s.storeDir, s.levels, sel)
	if err != nil {
		return nil, err
	}
	return &Tree{
		Root: root,
	}, nil
}

func lsLevelDir(
	dir string, levels []*Level, sel TarTypeMask,
) (*LevelTree, error) {
	subDirs, err := lsTimestampDirs(dir)
	if err != nil {
		return nil, err
	}

	times := make([]*TimeTree, 0, len(subDirs))
	for _, sub := range subDirs {
		tnode, err := lsTimeDir(dir, sub, levels, sel)
		if err != nil {
			return nil, err
		}
		if tnode != nil {
			times = append(times, tnode)
		}
	}
	if len(times) == 0 {
		// Return missing times as nil node without error.
		return nil, nil
	}

	return &LevelTree{
		name:  filepath.Base(dir),
		level: levels[0],
		Times: times,
	}, nil
}

func lsTimeDir(
	dir string, ts string, levels []*Level, sel TarTypeMask,
) (*TimeTree, error) {
	tsTime, err := time.Parse(timestampTimeFormat2, ts)
	if err != nil {
		t, err2 := time.Parse(timestampTimeFormat1, ts)
		if err2 != nil {
			return nil, err
		}
		tsTime = t
	}

	tarType := TarTypeFromTsDir(filepath.Join(dir, ts))
	if tarType == TarUnspecified {
		// Return missing tar as nil node without error.
		return nil, nil
	}
	if sel&TarTypeMask(tarType) == 0 {
		// Return other tar as nil node without error.
		return nil, nil
	}

	subLevels := levels[1:]
	children := make(map[string]*LevelTree)
	for i, slv := range subLevels {
		subName := slv.Name()
		subDir := filepath.Join(dir, ts, subName)
		if isDir(subDir) {
			child, err := lsLevelDir(subDir, subLevels[i:], sel)
			if err != nil {
				return nil, err
			}
			if child != nil {
				children[subName] = child
			}
		}
	}

	return &TimeTree{
		name:      ts,
		Time:      tsTime,
		level:     levels[0],
		TarType:   tarType,
		SubLevels: children,
	}, nil
}

// The v1 legacy format used a mixture of the ISO 8601 extended date format and
// basic time format, with UTC `Z`.  ISO 8601 does not allow this combination.
// Date and time must either use both the basic format or both the extended
// format.
//
// The v2 format uses the ISO 8601 basic format for both date and time.
//
// `tartt` now creates v2, but still reads v1 and v2.  Backwards compatibility
// with v1 should be maintained forever.
//
// Note that lexicographical sorting can be used for lists that contain both v1
// and v2 strings, because all v2 strings are greater than v1 strings, because
// the digit characters are greater than `-` in ASCII and UTF-8.  Assuming we
// do not write new v1 strings after the switch to v2, archives continue to
// sort correctly.
const (
	timestampTimeFormat1 = "2006-01-02T150405Z"
	timestampTimeFormat2 = "20060102T150405Z"
)

var timestampRgx = regexp.MustCompile(regexpx.Verbose(`
	^
	( [0-9]{4}-[0-9]{2}-[0-9]{2} | [0-9]{8} )
	T
	[0-9]{6}
	Z
	$
`))

func isTimestampString(s string) bool {
	if !timestampRgx.MatchString(s) {
		return false
	}
	for _, f := range []string{
		timestampTimeFormat2,
		timestampTimeFormat1,
	} {
		if _, err := time.Parse(f, s); err == nil {
			return true
		}
	}
	return false
}

func lsTimestampDirs(dir string) ([]string, error) {
	fp, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	ents, err := fp.Readdir(-1)
	_ = fp.Close()
	if err != nil {
		return nil, err
	}

	ts := make([]string, 0, len(ents))
	for _, e := range ents {
		name := e.Name()
		if isTimestampString(name) && e.IsDir() {
			ts = append(ts, name)
		}
	}
	sort.Strings(ts)

	return ts, nil
}

type TreePath []TreeNode

var TreePathEmpty TreePath = nil

func (path TreePath) DirPath() string {
	if len(path) == 0 {
		return ""
	}
	names := make([]string, 0, len(path))
	for _, p := range path {
		names = append(names, p.Name())
	}
	return slashpath.Join(names...)
}

func (path TreePath) TarPath() string {
	if len(path) == 0 {
		return ""
	}
	return slashpath.Join(
		path.DirPath(),
		path[len(path)-1].(*TimeTree).TarType.Path(),
	)
}

type AppendLocation struct {
	Parent  TreePath
	Now     time.Time
	Level   *Level
	TarType TarType
}

func (loc AppendLocation) ParentTarPath() string {
	if len(loc.Parent) == 0 {
		return ""
	}
	return loc.Parent.TarPath()
}

func (loc AppendLocation) TarPath() string {
	nowName := loc.Now.Format(timestampTimeFormat2)
	if len(loc.Parent) == 0 {
		if loc.TarType != TarFull {
			panic("wrong TarType without parent")
		}
		return slashpath.Join(nowName, loc.TarType.Path())
	}

	p := loc.Parent.DirPath()
	if loc.Level == loc.Parent[len(loc.Parent)-1].Level() {
		p = slashpath.Join(p, "..", nowName)
		p = slashpath.Clean(p)
	} else {
		p = slashpath.Join(p, loc.Level.Name(), nowName)
	}
	return slashpath.Join(p, loc.TarType.Path())
}

func (s *Store) WhereAppendFull(now time.Time) (AppendLocation, error) {
	return AppendLocation{
		Now:     now,
		Level:   s.levels[0],
		TarType: TarFull,
	}, nil
}

func (s *Store) WhereAppend(
	now time.Time, tree *Tree,
) (AppendLocation, error) {
	loc, err := tree.whereAppend(now, s.levels)
	loc.Now = now
	if loc.Level == s.levels[0] {
		loc.TarType = TarFull
	} else {
		loc.TarType = TarPatch
	}
	return loc, err
}

func (t *Tree) whereAppend(
	now time.Time, levels []*Level,
) (AppendLocation, error) {
	if t.Root == nil {
		return AppendLocation{
			Level: levels[0],
		}, nil
	}
	return t.Root.whereAppend(now, levels)
}

func (t *LevelTree) whereAppend(
	now time.Time, levels []*Level,
) (AppendLocation, error) {
	if t.level != levels[0] {
		err := errors.New("level mismatch")
		return AppendLocation{}, err
	}

	latest := t.Times[len(t.Times)-1]
	loc, err := latest.whereAppend(now, levels)
	if err != nil {
		return AppendLocation{}, err
	}
	latestPath := TreePath([]TreeNode{latest})
	loc.Parent = append(latestPath, loc.Parent...)
	return loc, nil
}

func (t *TimeTree) whereAppend(
	now time.Time, levels []*Level,
) (AppendLocation, error) {
	for i, lv := range levels {
		// Do not add new tars to a disabled level.  The first and last
		// level are always enabled, see `newStorePartialInit()`, so
		// that `whereAppend()` will terminate successfully.
		if lv.disabled {
			continue
		}
		lvName := lv.Name()
		if sub, ok := t.SubLevels[lvName]; ok {
			loc, err := sub.whereAppend(now, levels[i:])
			if err != nil {
				return AppendLocation{}, err
			}
			subPath := TreePath([]TreeNode{sub})
			loc.Parent = append(subPath, loc.Parent...)
			return loc, nil
		}

		next := lv.interval.AddTime(t.Time)
		if now.After(next) {
			return AppendLocation{
				Parent: TreePathEmpty,
				Level:  lv,
			}, nil
		}
	}
	err := fmt.Errorf(
		"possible clock skew: "+
			"now not in next interval for any level of tree %s/%s",
		t.level.Name(), t.Time.Format(timestampTimeFormat2),
	)
	return AppendLocation{}, err
}
