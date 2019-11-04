package getent

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

type Passwd struct {
	User string
	Uid  uint32
	Gid  uint32
}

func Passwds(ctx context.Context) ([]Passwd, error) {
	out, err := exec.CommandContext(
		ctx, getentTool.Path, "passwd",
	).Output()
	if err != nil {
		return nil, &ExecError{What: "getent passwd", Err: err}
	}
	txt := strings.TrimSpace(string(out))

	ps := make([]Passwd, 0)
	for _, line := range strings.Split(txt, "\n") {
		fs := strings.SplitN(line, ":", 5)
		if len(fs) != 5 {
			return nil, &ParseError{
				What: "getent passwd output",
				Text: line,
			}
		}
		fUser := fs[0]
		fUid := fs[2]
		fGid := fs[3]
		uid, err := strconv.ParseUint(fUid, 10, 32)
		if err != nil {
			return nil, &ParseError{
				What: "getent passwd output UID",
				Text: fUid,
			}
		}
		gid, err := strconv.ParseUint(fGid, 10, 32)
		if err != nil {
			return nil, &ParseError{
				What: "getent passwd output GID",
				Text: fGid,
			}
		}
		ps = append(ps, Passwd{
			User: fUser,
			Uid:  uint32(uid),
			Gid:  uint32(gid),
		})
	}

	return ps, nil
}

func SelectPasswds(pwds []Passwd, groups []Group) []Passwd {
	gids := make(map[uint32]struct{})
	for _, g := range groups {
		gids[g.Gid] = struct{}{}
	}

	res := make([]Passwd, 0, len(pwds))
	for _, p := range pwds {
		if _, ok := gids[p.Gid]; ok {
			res = append(res, p)
		}
	}
	return res
}
