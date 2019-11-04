package grpcjwt

import (
	"context"
	"errors"
	"io/ioutil"
	"strings"
)

var ErrMalformedJWT = errors.New("malformed JWT")

type FixedJWT struct {
	Token string
	AllowInsecureTransport bool
}

func (c *FixedJWT) RequireTransportSecurity() bool {
	return !c.AllowInsecureTransport
}

func (c *FixedJWT) GetRequestMetadata(
	ctx context.Context, uri ...string,
) (map[string]string, error) {
	return map[string]string{"authorization": c.Token}, nil
}

func Load(path string) (*FixedJWT, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// `jwt-go` does not support token parsing without signature
	// validation.  So we only do a minimal syntax check here.
	token := strings.TrimSpace(string(data))
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrMalformedJWT
	}

	return &FixedJWT{Token: token}, nil
}
