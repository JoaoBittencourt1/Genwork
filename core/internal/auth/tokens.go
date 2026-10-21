package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	AccessTokenLifetime = 15 * time.Minute
	TokenAudience       = "genwork"
)

var errInvalidAccessToken = errors.New("invalid access token")

// AccessClaims.Permissions lets modules authorize requests without calling core (rule 57).
type AccessClaims struct {
	SessionToken string   `json:"sid"`
	Permissions  []string `json:"permissions"`
	jwt.RegisteredClaims
}

type AccessTokenIssuer struct {
	key    SigningKey
	issuer string
	now    func() time.Time
}

func NewAccessTokenIssuer(key SigningKey, issuer string, now func() time.Time) *AccessTokenIssuer {
	return &AccessTokenIssuer{key: key, issuer: issuer, now: now}
}

func (issuer *AccessTokenIssuer) Issue(userToken, sessionToken string, permissions []string) (string, time.Time, error) {
	issuedAt := issuer.now()
	expiresAt := issuedAt.Add(AccessTokenLifetime)
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, AccessClaims{
		SessionToken: sessionToken,
		Permissions:  permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer.issuer,
			Audience:  jwt.ClaimStrings{TokenAudience},
			Subject:   userToken,
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	})
	token.Header["kid"] = issuer.key.keyID
	signed, err := token.SignedString(issuer.key.private)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

func (issuer *AccessTokenIssuer) Verify(rawToken string) (AccessClaims, error) {
	var claims AccessClaims
	_, err := jwt.ParseWithClaims(rawToken, &claims,
		func(*jwt.Token) (any, error) { return issuer.key.public, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(issuer.issuer),
		jwt.WithAudience(TokenAudience),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(issuer.now),
	)
	if err != nil || claims.Subject == "" || claims.SessionToken == "" {
		return AccessClaims{}, errInvalidAccessToken
	}
	return claims, nil
}
