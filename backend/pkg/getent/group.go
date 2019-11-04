package getent

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

type Group struct {
	Group string
	Gid   uint32
	Users []string
}

func (a Group) IsEqual(b Group) bool {
	if a.Group != b.Group {
		return false
	}
	if a.Gid != b.Gid {
		return false
	}
	if len(a.Users) != len(b.Users) {
		return false
	}
	for i, u := range a.Users {
		if u != b.Users[i] {
			return false
		}
	}
	return true
}

// `Groups()` returns a list of Unix groups as reported by `getent`.  The list
// may contain duplicates, even conflicting ones.
func Groups(ctx context.Context) ([]Group, error) {
	out, err := exec.CommandContext(ctx, getentTool.Path, "group").Output()
	if err != nil {
		return nil, &ExecError{What: "getent group", Err: err}
	}
	txt := strings.TrimSpace(string(out))

	gs := make([]Group, 0)
	for _, line := range strings.Split(txt, "\n") {
		fs := strings.Split(line, ":")
		if len(fs) != 4 {
			return nil, &ParseError{
				What: "getent group output",
				Text: line,
			}
		}
		fGroup := fs[0]
		fGid := fs[2]
		fUsers := fs[3]
		gid, err := strconv.ParseUint(fGid, 10, 32)
		if err != nil {
			return nil, &ParseError{
				What: "getent group output GID",
				Text: fGid,
			}
		}
		var users []string
		if fUsers != "" {
			users = strings.Split(fUsers, ",")
		}
		gs = append(gs, Group{
			Group: fGroup,
			Gid:   uint32(gid),
			Users: users,
		})
	}

	return gs, nil
}

// `SelectGroups()` selects `groups` whose names begin with any of the
// `prefixes`.
func SelectGroups(
	groups []Group,
	prefixes []string,
) []Group {
	hasPrefix := func(s string) bool {
		for _, p := range prefixes {
			if strings.HasPrefix(s, p) {
				return true
			}
		}
		return false
	}

	res := make([]Group, 0)
	for _, g := range groups {
		if hasPrefix(g.Group) {
			res = append(res, g)
		}
	}
	return res
}

// `DedupGroups()` returns a list without duplicate groups.  It returns an
// error if the input contains conflicting duplicates.
func DedupGroups(groups []Group) ([]Group, error) {
	byName := make(map[string]Group)
	byGid := make(map[uint32]Group)

	errConflict := func(a, b Group) error {
		return &GroupConflictError{
			AGroup: a.Group,
			AGid:   a.Gid,
			BGroup: b.Group,
			BGid:   b.Gid,
		}
	}

	isDuplicate := func(g Group) (bool, error) {
		if seen, ok := byName[g.Group]; ok {
			if !g.IsEqual(seen) {
				return true, errConflict(seen, g)
			}
			return true, nil
		}
		if seen, ok := byGid[g.Gid]; ok {
			if !g.IsEqual(seen) {
				return true, errConflict(seen, g)
			}
			return true, nil
		}
		return false, nil
	}

	res := make([]Group, 0)
	for _, g := range groups {
		isDup, err := isDuplicate(g)
		if err != nil {
			return nil, err
		}
		if isDup {
			continue
		}
		byName[g.Group] = g
		byGid[g.Gid] = g
		res = append(res, g)
	}

	return res, nil
}
