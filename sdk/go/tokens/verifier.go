// Package tokens validates core-issued access tokens with core's public keys, so modules never store credentials (rule 57).
package tokens

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	DefaultAudience           = "genwork"
	DefaultMinRefetchInterval = time.Minute
)

var (
	ErrInvalidToken    = errors.New("invalid access token")
	ErrKeysUnavailable = errors.New("core signing keys unavailable")
)

type JSONWebKey struct {
	KeyType   string `json:"kty"`
	Curve     string `json:"crv"`
	X         string `json:"x"`
	KeyID     string `json:"kid"`
	Algorithm string `json:"alg"`
	Use       string `json:"use"`
}

type JSONWebKeySet struct {
	Keys []JSONWebKey `json:"keys"`
}

// AccessClaims mirrors the claims core puts in every access token.
type AccessClaims struct {
	SessionToken string   `json:"sid"`
	Permissions  []string `json:"permissions"`
	jwt.RegisteredClaims
}

type Principal struct {
	UserToken   string
	Permissions []string
	AccessToken string
}

func (principal Principal) HasPermission(name string) bool {
	for _, granted := range principal.Permissions {
		if granted == name {
			return true
		}
	}
	return false
}

type TokenVerifierConfig struct {
	JWKSURL            string
	Issuer             string
	Audience           string
	HTTPClient         *http.Client
	Now                func() time.Time
	MinRefetchInterval time.Duration
}

type TokenVerifier struct {
	config    TokenVerifierConfig
	mutex     sync.Mutex
	keys      map[string]ed25519.PublicKey
	fetchedAt time.Time
}

func NewTokenVerifier(config TokenVerifierConfig) *TokenVerifier {
	if config.Audience == "" {
		config.Audience = DefaultAudience
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.MinRefetchInterval == 0 {
		config.MinRefetchInterval = DefaultMinRefetchInterval
	}
	return &TokenVerifier{config: config, keys: map[string]ed25519.PublicKey{}}
}

func (verifier *TokenVerifier) Verify(ctx context.Context, rawToken string) (Principal, error) {
	var claims AccessClaims
	_, err := jwt.ParseWithClaims(rawToken, &claims,
		func(token *jwt.Token) (any, error) {
			keyID, _ := token.Header["kid"].(string)
			return verifier.findKey(ctx, keyID)
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(verifier.config.Issuer),
		jwt.WithAudience(verifier.config.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(verifier.config.Now),
	)
	if errors.Is(err, ErrKeysUnavailable) {
		return Principal{}, ErrKeysUnavailable
	}
	if err != nil || claims.Subject == "" {
		return Principal{}, ErrInvalidToken
	}
	return Principal{UserToken: claims.Subject, Permissions: claims.Permissions, AccessToken: rawToken}, nil
}

// findKey refetches the key set on an unknown key ID, at most once per MinRefetchInterval, so forged key IDs cannot flood core.
func (verifier *TokenVerifier) findKey(ctx context.Context, keyID string) (ed25519.PublicKey, error) {
	verifier.mutex.Lock()
	defer verifier.mutex.Unlock()
	if key, found := verifier.keys[keyID]; found {
		return key, nil
	}
	now := verifier.config.Now()
	if !verifier.fetchedAt.IsZero() && now.Sub(verifier.fetchedAt) < verifier.config.MinRefetchInterval {
		return nil, ErrInvalidToken
	}
	keys, err := verifier.fetchKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKeysUnavailable, err)
	}
	verifier.keys = keys
	verifier.fetchedAt = now
	if key, found := keys[keyID]; found {
		return key, nil
	}
	return nil, ErrInvalidToken
}

func (verifier *TokenVerifier) fetchKeys(ctx context.Context) (map[string]ed25519.PublicKey, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, verifier.config.JWKSURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := verifier.config.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks status %d", response.StatusCode)
	}
	var keySet JSONWebKeySet
	if err := json.NewDecoder(response.Body).Decode(&keySet); err != nil {
		return nil, fmt.Errorf("decode jwks: %w", err)
	}
	keys := map[string]ed25519.PublicKey{}
	for _, key := range keySet.Keys {
		if key.KeyType != "OKP" || key.Curve != "Ed25519" {
			continue
		}
		decoded, err := base64.RawURLEncoding.DecodeString(key.X)
		if err != nil || len(decoded) != ed25519.PublicKeySize {
			continue
		}
		keys[key.KeyID] = decoded
	}
	return keys, nil
}
