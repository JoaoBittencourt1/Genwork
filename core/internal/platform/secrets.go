package platform

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

const bearerSecretBytes = 32

func RandomBytes(length int) []byte {
	buffer := make([]byte, length)
	_, _ = rand.Read(buffer)
	return buffer
}

func NewBearerSecret() string {
	return base64.RawURLEncoding.EncodeToString(RandomBytes(bearerSecretBytes))
}

func HashBearerSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}
