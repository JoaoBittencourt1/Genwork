package users_test

import (
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
	"strings"
	"testing"
)

var fastParams = users.PasswordParams{MemoryKiB: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

func TestHashUsesArgon2idPHCFormatWithUniqueSalt(t *testing.T) {
	hasher := users.NewPasswordHasher(fastParams, 2)
	first := hasher.Hash("correct horse battery")
	second := hasher.Hash("correct horse battery")

	if !strings.HasPrefix(first, "$argon2id$v=19$m=8192,t=1,p=1$") {
		t.Fatalf("unexpected encoding: %s", first)
	}
	if first == second {
		t.Error("two hashes of the same password must differ (unique salt)")
	}
	if strings.Contains(first, "correct horse battery") {
		t.Error("hash must not contain the password")
	}
}

func TestVerifyMatchesOnlyTheRightPassword(t *testing.T) {
	hasher := users.NewPasswordHasher(fastParams, 2)
	encoded := hasher.Hash("correct horse battery")

	if matches, _, err := hasher.Verify("correct horse battery", encoded); err != nil || !matches {
		t.Fatalf("expected match, got matches=%v err=%v", matches, err)
	}
	if matches, needsRehash, err := hasher.Verify("wrong horse battery", encoded); err != nil || matches || needsRehash {
		t.Fatalf("expected mismatch without rehash, got matches=%v needsRehash=%v err=%v", matches, needsRehash, err)
	}
}

func TestVerifyFlagsHashesWithWeakerParameters(t *testing.T) {
	weak := users.NewPasswordHasher(fastParams, 1).Hash("correct horse battery")
	stronger := fastParams
	stronger.MemoryKiB = 16 * 1024
	hasher := users.NewPasswordHasher(stronger, 1)

	matches, needsRehash, err := hasher.Verify("correct horse battery", weak)
	if err != nil || !matches || !needsRehash {
		t.Fatalf("expected match with rehash, got matches=%v needsRehash=%v err=%v", matches, needsRehash, err)
	}
	if _, needsRehash, _ := hasher.Verify("correct horse battery", hasher.Hash("correct horse battery")); needsRehash {
		t.Error("a current hash must not need rehash")
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	hasher := users.NewPasswordHasher(fastParams, 1)
	for _, encoded := range []string{"", "plain", "$argon2i$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA", "$argon2id$v=19$m=x$c2FsdA$aGFzaA"} {
		if _, _, err := hasher.Verify("whatever password", encoded); err == nil {
			t.Errorf("expected error for %q", encoded)
		}
	}
}

func TestDefaultParametersMeetTheConstitutionBaseline(t *testing.T) {
	if users.DefaultPasswordParams.MemoryKiB < 64*1024 || users.DefaultPasswordParams.Iterations < 3 || users.DefaultPasswordParams.Parallelism < 2 {
		t.Fatalf("default parameters below rule 66 baseline: %+v", users.DefaultPasswordParams)
	}
}

func TestValidatePasswordLengthCountsCodePoints(t *testing.T) {
	cases := map[string]bool{
		strings.Repeat("a", 7):   false,
		strings.Repeat("a", 8):   true,
		strings.Repeat("ç", 8):   true,
		strings.Repeat("a", 128): true,
		strings.Repeat("a", 129): false,
	}
	for password, valid := range cases {
		if err := users.ValidatePasswordLength(password); (err == nil) != valid {
			t.Errorf("ValidatePasswordLength(%d runes) error=%v, want valid=%v", len([]rune(password)), err, valid)
		}
	}
}

func TestNormalizeAndValidateEmail(t *testing.T) {
	if users.NormalizeEmail("  Ana@Example.COM ") != "ana@example.com" {
		t.Error("e-mail must be trimmed and lower-cased")
	}
	for email, valid := range map[string]bool{
		"ana@example.com":       true,
		"ana":                   false,
		"Ana <ana@example.com>": false,
		"":                      false,
	} {
		if users.IsValidEmail(email) != valid {
			t.Errorf("IsValidEmail(%q) = %v", email, !valid)
		}
	}
}
