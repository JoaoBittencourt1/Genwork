package githubsync

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strconv"
	"time"
)

const (
	// appJWTLifetime stays under GitHub's 10-minute cap; appJWTClockSkew backdates iat as GitHub recommends.
	appJWTLifetime  = 9 * time.Minute
	appJWTClockSkew = time.Minute
)

var ErrPrivateKeyInvalid = errors.New("the GitHub App private key is not a valid RSA key")

// SignAppJWT authenticates as the App itself (RS256, issuer = App id), the only way to ask for installation tokens.
func SignAppJWT(privateKey GithubCredential, githubAppID int64, now time.Time) (string, error) {
	key, err := parseRSAPrivateKey(privateKey.Reveal())
	if err != nil {
		return "", err
	}
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(map[string]any{
		"iat": now.Add(-appJWTClockSkew).Unix(), "exp": now.Add(appJWTLifetime).Unix(), "iss": strconv.FormatInt(githubAppID, 10),
	})
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign the App JWT: %w", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

// parseRSAPrivateKey reads the PKCS#1 PEM GitHub issues, and PKCS#8 for keys converted by hand.
func parseRSAPrivateKey(encoded []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(encoded)
	if block == nil {
		return nil, ErrPrivateKeyInvalid
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, ErrPrivateKeyInvalid
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrPrivateKeyInvalid
	}
	return key, nil
}
