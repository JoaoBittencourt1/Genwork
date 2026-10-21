package auth_test

import (
	"errors"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/auth"
	"github.com/zalando/go-keyring"
)

const instanceURL = "https://genwork.example.test"

func TestKeyringCredentialStoreRoundTripsOneCredentialPerInstance(t *testing.T) {
	keyring.MockInit()
	store := auth.NewKeyringCredentialStore()
	credential := auth.StoredCredential{Email: "ada@example.test", Session: "ses_1", RefreshToken: "refresh-1"}

	if err := store.Save(instanceURL, credential); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := store.Load(instanceURL)
	if err != nil || loaded != credential {
		t.Fatalf("Load = %+v, %v", loaded, err)
	}
	if _, err := store.Load("https://other.example.test"); !errors.Is(err, auth.ErrNotSignedIn) {
		t.Fatalf("another instance shares the credential: %v", err)
	}
}

func TestKeyringCredentialStoreDeleteIsIdempotent(t *testing.T) {
	keyring.MockInit()
	store := auth.NewKeyringCredentialStore()
	_ = store.Save(instanceURL, auth.StoredCredential{RefreshToken: "refresh-1"})

	if err := store.Delete(instanceURL); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := store.Delete(instanceURL); err != nil {
		t.Fatalf("second Delete: %v", err)
	}
	if _, err := store.Load(instanceURL); !errors.Is(err, auth.ErrNotSignedIn) {
		t.Fatalf("Load after Delete: %v", err)
	}
}

func TestKeyringCredentialStoreTreatsAnUnreadableSecretAsSignedOut(t *testing.T) {
	keyring.MockInit()
	_ = keyring.Set(auth.KeyringService, instanceURL, "not json")

	if _, err := auth.NewKeyringCredentialStore().Load(instanceURL); !errors.Is(err, auth.ErrNotSignedIn) {
		t.Fatalf("err = %v, want ErrNotSignedIn", err)
	}
}

func TestKeyringCredentialStoreReportsAnUnavailableSecureStore(t *testing.T) {
	unavailable := errors.New("secure store locked")
	keyring.MockInitWithError(unavailable)
	store := auth.NewKeyringCredentialStore()

	if _, err := store.Load(instanceURL); !errors.Is(err, unavailable) {
		t.Fatalf("Load err = %v", err)
	}
	if err := store.Save(instanceURL, auth.StoredCredential{}); !errors.Is(err, unavailable) {
		t.Fatalf("Save err = %v", err)
	}
	if err := store.Delete(instanceURL); !errors.Is(err, unavailable) {
		t.Fatalf("Delete err = %v", err)
	}
}
