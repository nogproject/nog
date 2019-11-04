package jwtauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/nogproject/nog/backend/pkg/auth"
	"github.com/nogproject/nog/backend/pkg/uuid"
)

var ErrAuthRequestNotOK = errors.New("auth request status not 200 OK")
var ErrUnknownScopeType = errors.New("scope of unknown type")

// `ScopeJWT` implements the GRPC `credentials.PerRPCCredentials` interface.
type ScopedJWT struct {
	Token string
}

type RepoIdScope struct {
	Action auth.Action `json:"action"`
	RepoId uuid.I      `json:"repoId"`
}

func (c *ScopedJWT) RequireTransportSecurity() bool {
	return true
}

func (c *ScopedJWT) GetRequestMetadata(
	ctx context.Context, uri ...string,
) (map[string]string, error) {
	return map[string]string{"authorization": c.Token}, nil
}

var httpClient = http.Client{
	Timeout: 20 * time.Second,
}

func GetScopedJWT(
	authURL, jwt string, scopes []interface{},
) (*ScopedJWT, error) {
	for _, sc := range scopes {
		switch sc.(type) {
		case auth.SimpleScope: // ok
		case RepoIdScope: // ok
		default:
			return nil, ErrUnknownScopeType
		}
	}
	return get(authURL, jwt, scopes)
}

func GetScopedJWTSimpleAndRepoId(
	authURL, jwt string,
	simpleScopes []auth.SimpleScope,
	idScopes []RepoIdScope,
) (*ScopedJWT, error) {
	scopes := make([]interface{}, 0, len(simpleScopes)+len(idScopes))
	for _, sc := range simpleScopes {
		scopes = append(scopes, sc)
	}
	for _, sc := range idScopes {
		scopes = append(scopes, sc)
	}
	return get(authURL, jwt, scopes)
}

func GetScopedJWTRepoId(
	authURL, jwt string, action auth.Action, repoId uuid.I,
) (*ScopedJWT, error) {
	scope := RepoIdScope{
		Action: action,
		RepoId: repoId,
	}
	return get(authURL, jwt, []interface{}{scope})
}

func get(authURL, jwt string, scopes interface{}) (*ScopedJWT, error) {
	iData := new(bytes.Buffer)
	if err := json.NewEncoder(iData).Encode(struct {
		ExpiresIn int         `json:"expiresIn"` // seconds
		Scopes    interface{} `json:"scopes"`
	}{
		ExpiresIn: 600,
		Scopes:    scopes,
	}); err != nil {
		return nil, err
	}
	i, err := http.NewRequest("POST", authURL, iData)
	if err != nil {
		return nil, err
	}
	i.Header.Add("Content-Type", "application/json; charset=utf-8")
	i.Header.Add("Authorization", fmt.Sprintf("Bearer %s", jwt))
	i.Header.Add("User-Agent", "nogfsoctl")
	o, err := httpClient.Do(i)
	if err != nil {
		return nil, err
	}
	defer o.Body.Close()
	if o.StatusCode != 200 {
		var oErrBody struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(o.Body).Decode(&oErrBody)
		if oErrBody.Message == "" {
			return nil, ErrAuthRequestNotOK
		}
		err := fmt.Errorf(
			"%s: %s", ErrAuthRequestNotOK, oErrBody.Message,
		)
		return nil, err
	}
	var oBody struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(o.Body).Decode(&oBody); err != nil {
		return nil, err
	}

	return &ScopedJWT{Token: oBody.Data.Token}, nil
}
