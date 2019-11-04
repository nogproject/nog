package grpcjwt

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/nogproject/nog/backend/pkg/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	ErrMissingMetadata = status.Error(
		codes.Unauthenticated, "missing GRPC metadata",
	)
	ErrMissingAuthMeta = status.Error(
		codes.Unauthenticated, "missing GRPC authorization metadata",
	)
	ErrWrongSigningMethod = status.Error(
		codes.Unauthenticated, "wrong JWT signing method",
	)
	ErrMissingX5c = status.Error(
		codes.Unauthenticated, "missing JWT x5c header",
	)
	ErrMalformedX5c = status.Error(
		codes.Unauthenticated, "malformed JWT x5c header",
	)
	ErrInvalidX5cKeyUsage = status.Error(
		codes.Unauthenticated,
		"JWT x5c key usage does not include signing",
	)
	ErrWrongX5cOU = status.Error(
		codes.Unauthenticated, "JWT x5c header has wrong X.509 OU",
	)
	ErrInvalidIssuer = status.Error(
		codes.Unauthenticated, "invalid token issuer",
	)
	ErrInvalidAudience = status.Error(
		codes.Unauthenticated, "invalid token audience",
	)
	ErrMissingSubject = status.Error(
		codes.Unauthenticated, "missing token subject",
	)
	ErrMalformedXcrd = status.Error(
		codes.Unauthenticated, "malformed token claim `xcrd`",
	)
	ErrMalformedSan = status.Error(
		codes.Unauthenticated, "malformed token claim `san`",
	)
	ErrInvalidSan = status.Error(
		codes.Unauthenticated, "invalid token claim `san` entry",
	)
	ErrMalformedSc = status.Error(
		codes.Unauthenticated, "malformed token claim `sc`",
	)
	ErrUnknownScAction = status.Error(
		codes.Unauthenticated, "unknown `sc.aa` in token claim",
	)
)

type HMACAuthn struct {
	secret   []byte
	issuer   string
	audience string
}

func NewHMACAuthn(secret string) *HMACAuthn {
	return &HMACAuthn{
		secret:   []byte(secret),
		issuer:   "nogapp",
		audience: "fso",
	}
}

type RSAAuthn struct {
	ca       *x509.CertPool
	ou       string
	issuer   string
	audience string
}

func NewRSAAuthn(ca *x509.CertPool, ou string) *RSAAuthn {
	return &RSAAuthn{
		ca:       ca,
		ou:       ou,
		issuer:   "nogapp",
		audience: "fso",
	}
}

type fsoClaims struct {
	jwt.MapClaims
}

func (c *fsoClaims) VerifyIssuer(expected string) bool {
	const yesRequired = true
	return c.MapClaims.VerifyIssuer(expected, yesRequired)
}

func (c *fsoClaims) VerifyAudience(expected string) bool {
	aud, _ := c.MapClaims["aud"].([]interface{})
	for _, a := range aud {
		val, _ := a.(string)
		if val == expected {
			return true
		}
	}
	return false
}

func (c *fsoClaims) Subject() (string, error) {
	sub, _ := c.MapClaims["sub"].(string)
	if sub == "" {
		return "", ErrMissingSubject
	}
	return sub, nil
}

func (c *fsoClaims) Xcrd() (auth.UnixIdentities, error) {
	xcrd, ok := c.MapClaims["xcrd"]
	if !ok {
		return nil, nil
	}

	ids, ok := asUnixIdentities(xcrd)
	if !ok {
		return nil, ErrMalformedXcrd
	}

	return ids, nil
}

func asUnixIdentities(in interface{}) (auth.UnixIdentities, bool) {
	vals, ok := in.([]interface{})
	if !ok {
		return nil, false
	}

	ids := make([]auth.UnixIdentity, 0, len(vals))
	for _, val := range vals {
		m, ok := val.(map[string]interface{})
		if !ok {
			return nil, false
		}
		if len(m) != 3 {
			return nil, false
		}
		id := auth.UnixIdentity{}
		id.Domain, ok = m["d"].(string)
		if !ok {
			return nil, false
		}
		id.Username, ok = m["u"].(string)
		if !ok {
			return nil, false
		}
		id.Groupnames, ok = asStringList(m["g"])
		if !ok {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

func (c *fsoClaims) San() ([]string, error) {
	san, ok := c.MapClaims["san"]
	if !ok {
		return nil, nil
	}

	strs, ok := asStringList(san)
	if !ok {
		return nil, ErrMalformedSan
	}
	for _, s := range strs {
		if !strings.HasPrefix(s, "DNS:") {
			return nil, ErrInvalidSan
		}
	}

	return strs, nil
}

func (c *fsoClaims) Scopes() ([]auth.Scope, error) {
	sc, ok := c.MapClaims["sc"]
	if !ok {
		return nil, nil
	}

	vals, ok := sc.([]interface{})
	if !ok {
		return nil, ErrMalformedSc
	}
	if len(vals) == 0 {
		return nil, nil
	}

	out := make([]auth.Scope, 0, len(vals))
	for _, val := range vals {
		valMap, ok := val.(map[string]interface{})
		if !ok {
			return nil, ErrMalformedSc
		}
		// At least action `aa` and path `p` or name `n`, or all three.
		if len(valMap) < 2 {
			return nil, ErrMalformedSc
		}

		actions, ok := asStringList(valMap["aa"])
		if !ok {
			return nil, ErrMalformedSc
		}
		for i, a := range actions {
			action, ok := jwtAAToAccessAction[a]
			if !ok {
				return nil, ErrUnknownScAction
			}
			actions[i] = action
		}

		scope := auth.Scope{Actions: actions}

		if _, ok := valMap["p"]; ok {
			paths, ok := asStringList(valMap["p"])
			if !ok {
				return nil, ErrMalformedSc
			}
			scope.Paths = paths
		}

		if _, ok := valMap["n"]; ok {
			names, ok := asStringList(valMap["n"])
			if !ok {
				return nil, ErrMalformedSc
			}
			scope.Names = names
		}

		out = append(out, scope)
	}

	return out, nil
}

func asStringList(in interface{}) ([]string, bool) {
	vals, ok := in.([]interface{})
	if !ok {
		return nil, false
	}

	strs := make([]string, 0, len(vals))
	for _, val := range vals {
		str, ok := val.(string)
		if !ok {
			return nil, false
		}
		strs = append(strs, str)
	}

	return strs, true
}

func (a *HMACAuthn) Authenticate(ctx context.Context) (auth.Identity, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, ErrMissingMetadata
	}
	au, ok := md["authorization"]
	if !ok || len(au) < 1 {
		return nil, ErrMissingAuthMeta
	}

	var claims fsoClaims
	_, err := jwt.ParseWithClaims(au[0], &claims.MapClaims, a.keyfunc())
	if err != nil {
		return nil, status.Errorf(
			codes.Unauthenticated, "invalid token: %s", err,
		)
	}

	if !claims.VerifyIssuer(a.issuer) {
		return nil, ErrInvalidIssuer
	}

	if !claims.VerifyAudience(a.audience) {
		return nil, ErrInvalidAudience
	}

	var sub string
	sub, err = claims.Subject()
	if err != nil {
		return nil, err
	}

	var xcrd auth.UnixIdentities
	xcrd, err = claims.Xcrd()
	if err != nil {
		return nil, err
	}

	var san []string
	san, err = claims.San()
	if err != nil {
		return nil, err
	}

	var scopes []auth.Scope
	scopes, err = claims.Scopes()
	if err != nil {
		return nil, err
	}

	euid := map[string]interface{}{
		"subject": sub,
	}
	if len(xcrd) > 0 {
		euid["unix"] = xcrd
	}
	if len(san) > 0 {
		euid["san"] = san
	}
	if len(scopes) > 0 {
		euid["scopes"] = scopes
	}
	return euid, nil
}

func (a *HMACAuthn) keyfunc() jwt.Keyfunc {
	return func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrWrongSigningMethod
		}
		return a.secret, nil
	}
}

func (a *RSAAuthn) Authenticate(ctx context.Context) (auth.Identity, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, ErrMissingMetadata
	}
	au, ok := md["authorization"]
	if !ok || len(au) < 1 {
		return nil, ErrMissingAuthMeta
	}

	var claims fsoClaims
	_, err := jwt.ParseWithClaims(au[0], &claims.MapClaims, a.keyfunc())
	if err != nil {
		return nil, status.Errorf(
			codes.Unauthenticated, "invalid token: %s", err,
		)
	}

	if !claims.VerifyIssuer(a.issuer) {
		return nil, ErrInvalidIssuer
	}

	if !claims.VerifyAudience(a.audience) {
		return nil, ErrInvalidAudience
	}

	var sub string
	sub, err = claims.Subject()
	if err != nil {
		return nil, err
	}

	var xcrd auth.UnixIdentities
	xcrd, err = claims.Xcrd()
	if err != nil {
		return nil, err
	}

	var san []string
	san, err = claims.San()
	if err != nil {
		return nil, err
	}

	var scopes []auth.Scope
	scopes, err = claims.Scopes()
	if err != nil {
		return nil, err
	}

	euid := map[string]interface{}{
		"subject": sub,
	}
	if len(xcrd) > 0 {
		euid["unix"] = xcrd
	}
	if len(san) > 0 {
		euid["san"] = san
	}
	if len(scopes) > 0 {
		euid["scopes"] = scopes
	}
	return euid, nil
}

func (a *RSAAuthn) keyfunc() jwt.Keyfunc {
	return func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, ErrWrongSigningMethod
		}

		x5c, ok := t.Header["x5c"].(string)
		if !ok {
			return nil, ErrMissingX5c
		}

		der, err := base64.StdEncoding.DecodeString(x5c)
		if err != nil {
			return nil, ErrMalformedX5c
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, ErrMalformedX5c
		}

		if _, err := cert.Verify(x509.VerifyOptions{
			Roots:     a.ca,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		}); err != nil {
			err := status.Errorf(
				codes.Unauthenticated,
				"invalid x5c header: %s", err,
			)
			return nil, err
		}

		// See <https://tools.ietf.org/html/rfc5280#section-4.2.1.3>
		if cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
			return nil, ErrInvalidX5cKeyUsage
		}

		if len(cert.Subject.OrganizationalUnit) != 1 {
			return nil, ErrWrongX5cOU
		}
		if cert.Subject.OrganizationalUnit[0] != a.ou {
			return nil, ErrWrongX5cOU
		}

		return cert.PublicKey, nil
	}
}
