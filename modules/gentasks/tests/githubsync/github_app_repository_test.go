package githubsync_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boardtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsynctest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform/dbtest"
)

func TestGithubAppRepositoryStoresASingleApp(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := githubsync.GithubAppRepository{}
	stored := githubsynctest.StoreGithubApp(t, db, githubsynctest.EncryptionKey)

	found, err := repository.Find(ctx, db)

	if err != nil || found.Token != stored.Token || found.GithubAppID != 4242 || found.Slug != "genwork-fixture" || found.OwnerLogin != "acme" ||
		found.AppURL != "https://github.test/apps/genwork-fixture" || found.ConnectedBy != boardtest.CreatorToken || !found.CreatedAt.Equal(boardtest.FixedTime) {
		t.Fatalf("found = %+v, err = %v", found, err)
	}
	if len(found.WebhookSecretCiphertext) == 0 || len(found.PrivateKeyCiphertext) == 0 {
		t.Error("ciphertexts must be stored")
	}
	_, err = repository.Create(ctx, db, githubsync.NewGithubApp{
		Token: "second-app-token", GithubAppID: 1, Slug: "other", OwnerLogin: "other", AppURL: "https://github.test/apps/other",
		WebhookSecretCiphertext: []byte{1}, PrivateKeyCiphertext: []byte{1}, ConnectedBy: boardtest.CreatorToken, At: boardtest.FixedTime,
	})
	if !errors.Is(err, githubsync.ErrGithubAlreadyConnected) {
		t.Errorf("second app error = %v", err)
	}
}

func TestGithubAppRepositoryDeletesTheRowPhysically(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := githubsync.GithubAppRepository{}
	githubsynctest.StoreGithubApp(t, db, githubsynctest.EncryptionKey)

	if err := repository.Delete(ctx, db); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repository.Delete(ctx, db); err != nil {
		t.Fatalf("delete when nothing is stored: %v", err)
	}

	if _, err := repository.Find(ctx, db); !errors.Is(err, githubsync.ErrGithubNotConnected) {
		t.Errorf("find after delete error = %v", err)
	}
	var rows int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM github_apps`).Scan(&rows); err != nil || rows != 0 {
		t.Errorf("rows = %d, err = %v; disconnecting must remove the credentials", rows, err)
	}
}
