package users

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"golang.org/x/crypto/argon2"
)

const (
	MinPasswordLength = 8
	MaxPasswordLength = 128
)

var ErrPasswordLength = platform.NewAPIError(http.StatusBadRequest, "password_length_invalid")

var errMalformedHash = errors.New("malformed password hash")

type PasswordParams struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

var DefaultPasswordParams = PasswordParams{
	MemoryKiB:   64 * 1024,
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

func (params PasswordParams) isWeakerThan(other PasswordParams) bool {
	return params.MemoryKiB < other.MemoryKiB ||
		params.Iterations < other.Iterations ||
		params.Parallelism < other.Parallelism ||
		params.KeyLength < other.KeyLength
}

func ValidatePasswordLength(password string) error {
	length := utf8.RuneCountInString(password)
	if length < MinPasswordLength || length > MaxPasswordLength {
		return ErrPasswordLength
	}
	return nil
}

// PasswordHasher bounds concurrent hashes because each Argon2id run holds MemoryKiB of RAM.
type PasswordHasher struct {
	params    PasswordParams
	slots     chan struct{}
	dummyHash string
}

func NewPasswordHasher(params PasswordParams, maxConcurrentHashes int) *PasswordHasher {
	hasher := &PasswordHasher{params: params, slots: make(chan struct{}, maxConcurrentHashes)}
	hasher.dummyHash = hasher.Hash(base64.RawStdEncoding.EncodeToString(platform.RandomBytes(16)))
	return hasher
}

func (hasher *PasswordHasher) Hash(password string) string {
	salt := platform.RandomBytes(int(hasher.params.SaltLength))
	key := hasher.derive(password, salt, hasher.params)
	return encodePHC(hasher.params, salt, key)
}

// Verify reports whether the password matches and whether the stored hash should be upgraded to current parameters.
func (hasher *PasswordHasher) Verify(password, encodedHash string) (matches bool, needsRehash bool, err error) {
	params, salt, expectedKey, err := decodePHC(encodedHash)
	if err != nil {
		return false, false, err
	}
	key := hasher.derive(password, salt, params)
	matches = subtle.ConstantTimeCompare(key, expectedKey) == 1
	return matches, matches && params.isWeakerThan(hasher.params), nil
}

// VerifyAgainstDummy spends the same work as a real verification so unknown accounts are not revealed by timing.
func (hasher *PasswordHasher) VerifyAgainstDummy(password string) {
	_, _, _ = hasher.Verify(password, hasher.dummyHash)
}

func (hasher *PasswordHasher) derive(password string, salt []byte, params PasswordParams) []byte {
	hasher.slots <- struct{}{}
	defer func() { <-hasher.slots }()
	return argon2.IDKey([]byte(password), salt, params.Iterations, params.MemoryKiB, params.Parallelism, params.KeyLength)
}

func encodePHC(params PasswordParams, salt, key []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, params.MemoryKiB, params.Iterations, params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

func decodePHC(encoded string) (PasswordParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return PasswordParams{}, nil, nil, errMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return PasswordParams{}, nil, nil, errMalformedHash
	}
	var params PasswordParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.MemoryKiB, &params.Iterations, &params.Parallelism); err != nil {
		return PasswordParams{}, nil, nil, errMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return PasswordParams{}, nil, nil, errMalformedHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return PasswordParams{}, nil, nil, errMalformedHash
	}
	params.SaltLength = uint32(len(salt))
	params.KeyLength = uint32(len(key))
	return params, salt, key, nil
}
