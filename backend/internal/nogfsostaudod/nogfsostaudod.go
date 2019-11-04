package nogfsostaudod

import (
	"fmt"
	"os"

	"github.com/nogproject/nog/backend/pkg/pwd"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrUserMismatch = status.Error(
	codes.InvalidArgument, "request `username` != server process user",
)

type UnknownUIDError struct {
	UID uint32
}

func (err *UnknownUIDError) Error() string {
	return fmt.Sprintf("unknown uid %d", err.UID)
}

type Logger interface {
	Infow(msg string, kv ...interface{})
	Warnw(msg string, kv ...interface{})
}

type Server struct {
	lg       Logger
	username string
}

func GetpwuidName(uid uint32) (string, error) {
	pw := pwd.Getpwuid(uid)
	if pw == nil {
		return "", &UnknownUIDError{UID: uid}
	}
	return pw.Name, nil
}

func New(lg Logger) (*Server, error) {
	uid := uint32(os.Getuid())
	username, err := GetpwuidName(uid)
	if err != nil {
		return nil, err
	}
	return &Server{
		lg:       lg,
		username: username,
	}, nil
}
