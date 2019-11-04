package main

import (
	"errors"
	"net"
	"syscall"
)

var ErrDefaultDeny = errors.New("default deny")

type Auther interface {
	Auth(*net.UnixConn) error
}

type nullAuther struct{}

func (auth *nullAuther) Auth(*net.UnixConn) error {
	return nil
}

type AnyUnixCredsAuther struct {
	Lg   Logger
	UIDs []uint32
	GIDs []uint32
}

func (auth *AnyUnixCredsAuther) Auth(conn *net.UnixConn) error {
	f, err := conn.File()
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	cred, err := syscall.GetsockoptUcred(
		int(f.Fd()), syscall.SOL_SOCKET, syscall.SO_PEERCRED,
	)
	if err != nil {
		return err
	}

	for _, gid := range auth.GIDs {
		if cred.Gid == gid {
			auth.Lg.Infow(
				"Allowed GID to connect.",
				"gid", gid,
			)
			return nil
		}
	}

	for _, uid := range auth.UIDs {
		if cred.Uid == uid {
			auth.Lg.Infow(
				"Allowed UID to connect.",
				"uid", uid,
			)
			return nil
		}
	}

	return ErrDefaultDeny
}
