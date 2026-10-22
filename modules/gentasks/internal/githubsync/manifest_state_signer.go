package githubsync

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const manifestStateLifetime = time.Hour

var ErrInvalidManifestState = errors.New("manifest state is invalid, expired, or issued to another user")

// ManifestStateSigner issues the state round-tripped through GitHub's manifest flow without storing it: the payload names
// the user and an expiry, and an HMAC proves gentasks issued it (design §4).
type ManifestStateSigner struct {
	key []byte
}

type manifestStatePayload struct {
	UserToken string `json:"u"`
	ExpiresAt int64  `json:"e"`
	Nonce     string `json:"n"`
}

func NewManifestStateSigner(encryptionKey []byte) (*ManifestStateSigner, error) {
	key, err := deriveKey(encryptionKey, manifestStateKeyInfo)
	if err != nil {
		return nil, err
	}
	return &ManifestStateSigner{key: key}, nil
}

func (signer *ManifestStateSigner) Issue(userToken string, now time.Time) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}
	payload, err := json.Marshal(manifestStatePayload{
		UserToken: userToken, ExpiresAt: now.Add(manifestStateLifetime).Unix(), Nonce: base64.RawURLEncoding.EncodeToString(nonce),
	})
	if err != nil {
		return "", fmt.Errorf("encode manifest state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signer.sign(payload)), nil
}

func (signer *ManifestStateSigner) Verify(state, userToken string, now time.Time) error {
	encodedPayload, encodedSignature, found := strings.Cut(state, ".")
	if !found {
		return ErrInvalidManifestState
	}
	payload, payloadErr := base64.RawURLEncoding.DecodeString(encodedPayload)
	signature, signatureErr := base64.RawURLEncoding.DecodeString(encodedSignature)
	if payloadErr != nil || signatureErr != nil || !hmac.Equal(signature, signer.sign(payload)) {
		return ErrInvalidManifestState
	}
	var decoded manifestStatePayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return ErrInvalidManifestState
	}
	if decoded.UserToken != userToken || !now.Before(time.Unix(decoded.ExpiresAt, 0)) {
		return ErrInvalidManifestState
	}
	return nil
}

func (signer *ManifestStateSigner) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, signer.key)
	mac.Write(payload)
	return mac.Sum(nil)
}
