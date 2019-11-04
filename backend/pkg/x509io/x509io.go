// Package `x509io` contains functions to load certs from disk.
package x509io

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"path/filepath"
)

// `LoadX509Dir()` loads a triplet `ca.pem`, `cert.pem`, and `privkey.pem`.
func LoadX509Dir(path string) (
	cert tls.Certificate, ca *x509.CertPool, err error,
) {
	// Resolve to realpath to be robust against concurrent cert updates,
	// which may swap a symlink.
	d, err := filepath.EvalSymlinks(path)
	if err != nil {
		return cert, nil, err
	}

	cert, err = tls.LoadX509KeyPair(
		filepath.Join(d, "cert.pem"),
		filepath.Join(d, "privkey.pem"),
	)
	if err != nil {
		return cert, nil, err
	}

	caPem, err := ioutil.ReadFile(filepath.Join(d, "ca.pem"))
	if err != nil {
		return cert, nil, err
	}
	ca = x509.NewCertPool()
	ca.AppendCertsFromPEM(caPem)

	return cert, ca, nil
}

// `LoadCombinedCert()` loads a combined cert and key.  PEM files can be
// concatenated `cat cert.pem privkey.pem > combined.pem`.
func LoadCombinedCert(path string) (cert tls.Certificate, err error) {
	pem, err := ioutil.ReadFile(path)
	if err != nil {
		return cert, err
	}
	// X509KeyPair() handles combined PEM.  It skips unexpected PEM blocks.
	return tls.X509KeyPair(pem, pem)
}

// `LoadCABundle()` loads PEM certificates as a `CertPool`, which can be used
// as a CA for client certs.
func LoadCABundle(path string) (*x509.CertPool, error) {
	pem, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ca := x509.NewCertPool()
	if !ca.AppendCertsFromPEM(pem) {
		err := fmt.Errorf("failed to parse certs from `%s`", path)
		return nil, err
	}
	return ca, nil
}
