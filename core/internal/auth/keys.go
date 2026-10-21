package auth

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

var errInvalidSigningKey = errors.New("JWT_SIGNING_KEY must be a base64-encoded 32-byte Ed25519 seed")

type SigningKey struct {
	private ed25519.PrivateKey
	public  ed25519.PublicKey
	keyID   string
}

func GenerateSigningKeySeed() string {
	return base64.StdEncoding.EncodeToString(platform.RandomBytes(ed25519.SeedSize))
}

func ParseSigningKey(encodedSeed string) (SigningKey, error) {
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedSeed))
	if err != nil || len(seed) != ed25519.SeedSize {
		return SigningKey{}, errInvalidSigningKey
	}
	private := ed25519.NewKeyFromSeed(seed)
	public := private.Public().(ed25519.PublicKey)
	fingerprint := sha256.Sum256(public)
	return SigningKey{
		private: private,
		public:  public,
		keyID:   base64.RawURLEncoding.EncodeToString(fingerprint[:12]),
	}, nil
}

type JWK struct {
	KeyType   string `json:"kty"`
	Curve     string `json:"crv"`
	X         string `json:"x"`
	KeyID     string `json:"kid"`
	Algorithm string `json:"alg"`
	Use       string `json:"use"`
}

type JWKSet struct {
	Keys []JWK `json:"keys"`
}

func (key SigningKey) JWKSet() JWKSet {
	return JWKSet{Keys: []JWK{{
		KeyType:   "OKP",
		Curve:     "Ed25519",
		X:         base64.RawURLEncoding.EncodeToString(key.public),
		KeyID:     key.keyID,
		Algorithm: "EdDSA",
		Use:       "sig",
	}}}
}
