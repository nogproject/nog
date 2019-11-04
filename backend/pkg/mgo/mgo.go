// Package `mgo` wraps `gopkg.in/mgo.v2`.
package mgo

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/url"

	"github.com/nogproject/nog/backend/pkg/x509io"
	mgo "gopkg.in/mgo.v2"
)

type Session = mgo.Session

// `Dial()` supports connecting to UNIX domain sockets.
func Dial(uri string) (*mgo.Session, error) {
	mgi, err := mgo.ParseURL(uri)
	if err != nil {
		err = fmt.Errorf("failed to parse: %s", err)
		return nil, err
	}

	// See MongoDB, Connection String URI Format, UNIX domain socket,
	// <https://goo.gl/65BdaT>.
	if len(mgi.Addrs) == 1 && mgi.Addrs[0][0] == '%' {
		path, err := url.PathUnescape(mgi.Addrs[0])
		if err != nil {
			err = fmt.Errorf(
				"failed to parse Unix socket path: %v", err,
			)
			return nil, err
		}
		mgi.Addrs = []string{"localhost"}
		mgi.DialServer = func(*mgo.ServerAddr) (net.Conn, error) {
			return net.Dial("unix", path)
		}
	}

	return mgo.DialWithInfo(mgi)
}

func DialCACert(uri, caPath, certPath string) (*mgo.Session, error) {
	mgi, err := mgo.ParseURL(uri)
	if err != nil {
		err = fmt.Errorf("failed to parse: %s", err)
		return nil, err
	}

	tlsCfg := &tls.Config{}
	if caPath != "" {
		ca, err := x509io.LoadCABundle(caPath)
		if err != nil {
			err = fmt.Errorf("failed to load CA: %s", err)
			return nil, err
		}
		tlsCfg.RootCAs = ca
	}
	if certPath != "" {
		cert, err := x509io.LoadCombinedCert(certPath)
		if err != nil {
			err = fmt.Errorf("failed to load certificate: %s", err)
			return nil, err
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	mgi.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
		return tls.Dial("tcp", addr.String(), tlsCfg)
	}

	return mgo.DialWithInfo(mgi)
}
