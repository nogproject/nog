package gitstat

import "fmt"

// `Mode` is a Linux/Git file mode.
type Mode uint32

// `Filetype` are the type bits of a `Mode m`, i.e. `m & ModeType`.
type Filetype Mode

// `Perms` are the permission bits of a `Mode m`, i.e. `m & ModePerms`.
type Perms Mode

const (
	ModePerms Mode = 0777 // Unix permission bits

	ModeType    Mode = 0170000 // type mask
	ModeDir     Mode = 0040000 // `d` dir
	ModeRegular Mode = 0100000 // `f` regular file
	ModeSymlink Mode = 0120000 // `l` symlink
	ModeGitlink Mode = 0160000 // `g` gitlink, aka submodule commit
)

func (m Mode) Filetype() Filetype {
	return Filetype(m & ModeType)
}

func (m Mode) Perms() Perms {
	return Perms(m & ModePerms)
}

func (m Mode) IsDir() bool {
	return m&ModeType == ModeDir
}

func (m Mode) IsRegular() bool {
	return m&ModeType == ModeRegular
}

func (m Mode) IsSymlink() bool {
	return m&ModeType == ModeSymlink
}

func (m Mode) IsGitlink() bool {
	return m&ModeType == ModeGitlink
}

func (m Mode) String() string {
	return fmt.Sprintf("%s %s", m.Filetype(), m.Perms())
}

func (m Filetype) String() string {
	switch Mode(m) {
	case ModeDir:
		return "d"
	case ModeRegular:
		return "f"
	case ModeSymlink:
		return "l"
	case ModeGitlink:
		return "g"
	default:
		return "?"
	}
}

func (m Perms) String() string {
	// %0#4o: zero-padded, enforce leading 0, width 4, octal.
	return fmt.Sprintf("%0#4o", m)
}
