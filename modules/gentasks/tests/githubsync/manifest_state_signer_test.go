package githubsync_test

import (
	"errors"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boardtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsynctest"
)

func newManifestStateSigner(t *testing.T, key []byte) *githubsync.ManifestStateSigner {
	t.Helper()
	signer, err := githubsync.NewManifestStateSigner(key)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	return signer
}

func TestManifestStateVerifiesForItsUserUntilItExpires(t *testing.T) {
	signer := newManifestStateSigner(t, githubsynctest.EncryptionKey)
	state, err := signer.Issue("ana", boardtest.FixedTime)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if err := signer.Verify(state, "ana", boardtest.FixedTime.Add(59*time.Minute)); err != nil {
		t.Errorf("fresh state: %v", err)
	}
	if err := signer.Verify(state, "ana", boardtest.FixedTime.Add(time.Hour)); !errors.Is(err, githubsync.ErrInvalidManifestState) {
		t.Errorf("expired state error = %v", err)
	}
	if err := signer.Verify(state, "bruno", boardtest.FixedTime); !errors.Is(err, githubsync.ErrInvalidManifestState) {
		t.Errorf("foreign state error = %v", err)
	}
}

func TestManifestStateRejectsForgedValues(t *testing.T) {
	signer := newManifestStateSigner(t, githubsynctest.EncryptionKey)
	state, _ := signer.Issue("ana", boardtest.FixedTime)
	forgedByOtherKey, _ := newManifestStateSigner(t, githubsynctest.OtherEncryptionKey).Issue("ana", boardtest.FixedTime)

	for _, forged := range []string{"", "no-separator", "a.b.c", "!!!.!!!", forgedByOtherKey, state + "x", "x" + state} {
		if err := signer.Verify(forged, "ana", boardtest.FixedTime); !errors.Is(err, githubsync.ErrInvalidManifestState) {
			t.Errorf("%q: error = %v", forged, err)
		}
	}
}

func TestManifestStatesAreUnique(t *testing.T) {
	signer := newManifestStateSigner(t, githubsynctest.EncryptionKey)
	first, _ := signer.Issue("ana", boardtest.FixedTime)
	second, _ := signer.Issue("ana", boardtest.FixedTime)
	if first == second {
		t.Error("two states issued at the same time must differ")
	}
}
