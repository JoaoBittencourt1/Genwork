package accounts_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/accounts"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform/dbtest"
)

var fixtureTime = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

func apiKeyAccount(name string, provider accounts.Provider) accounts.NewAiAccount {
	last4 := "cret"
	return accounts.NewAiAccount{
		Token: platform.NewPublicToken(), Name: name, Provider: provider, Mode: accounts.AccountModeAPIKey,
		APIKeyCiphertext: []byte("ciphertext"), APIKeyLast4: &last4, Status: accounts.AccountStatusReady, At: fixtureTime,
	}
}

func TestSeveralAccountsPerProviderWithUniqueNames(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := accounts.AiAccountRepository{}

	if _, err := repository.Create(ctx, db, apiKeyAccount("Claude — João", accounts.ProviderAnthropic)); err != nil {
		t.Fatalf("first account: %v", err)
	}
	if _, err := repository.Create(ctx, db, apiKeyAccount("Claude — Team", accounts.ProviderAnthropic)); err != nil {
		t.Fatalf("second account of the same provider: %v", err)
	}
	_, err := repository.Create(ctx, db, apiKeyAccount("Claude — Team", accounts.ProviderOpenAI))

	if !errors.Is(err, accounts.ErrAccountNameTaken) {
		t.Errorf("duplicate name error = %v", err)
	}
	listed, err := repository.List(ctx, db)
	if err != nil || len(listed) != 2 {
		t.Fatalf("listed = %d, err = %v", len(listed), err)
	}
}

func TestSoftDeleteClearsTheKeyAndFreesTheName(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := accounts.AiAccountRepository{}
	account, err := repository.Create(ctx, db, apiKeyAccount("Claude", accounts.ProviderAnthropic))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repository.SoftDelete(ctx, db, account.ID, fixtureTime); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	var ciphertext []byte
	var last4 *string
	if err := db.QueryRow(ctx, `SELECT api_key_ciphertext, api_key_last4 FROM ai_accounts WHERE id = $1`, account.ID).Scan(&ciphertext, &last4); err != nil {
		t.Fatalf("read stored account: %v", err)
	}
	if ciphertext != nil || last4 != nil {
		t.Errorf("removed account kept its key: %v %v", ciphertext, last4)
	}
	if _, err := repository.FindByID(ctx, db, account.ID); !errors.Is(err, accounts.ErrAccountNotFound) {
		t.Errorf("find removed account error = %v", err)
	}
	if _, err := repository.Create(ctx, db, apiKeyAccount("Claude", accounts.ProviderAnthropic)); err != nil {
		t.Errorf("reusing the name of a removed account: %v", err)
	}
}

func TestSubscriptionAccountNeedsARunner(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)

	_, err := accounts.AiAccountRepository{}.Create(context.Background(), db, accounts.NewAiAccount{
		Token: platform.NewPublicToken(), Name: "Claude", Provider: accounts.ProviderAnthropic, Mode: accounts.AccountModeSubscription,
		Status: accounts.AccountStatusPendingSignIn, At: fixtureTime,
	})

	if err == nil {
		t.Error("a subscription account without a runner was accepted")
	}
}

func TestReplaceAPIKeyReturnsTheAccountToReady(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := accounts.AiAccountRepository{}
	account, _ := repository.Create(ctx, db, apiKeyAccount("Claude", accounts.ProviderAnthropic))
	message := "invalid x-api-key"
	if err := repository.UpdateStatus(ctx, db, account.ID, accounts.AccountStatusError, &message, fixtureTime); err != nil {
		t.Fatalf("update status: %v", err)
	}

	if err := repository.ReplaceAPIKey(ctx, db, account.ID, []byte("new"), "9999", fixtureTime); err != nil {
		t.Fatalf("replace key: %v", err)
	}

	replaced, _ := repository.FindByID(ctx, db, account.ID)
	if replaced.Status != accounts.AccountStatusReady || replaced.LastError != nil || replaced.APIKeyLast4 == nil || *replaced.APIKeyLast4 != "9999" {
		t.Errorf("replaced account = %+v", replaced)
	}
}

func TestRenameAccountKeepsNamesUnique(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	repository := accounts.AiAccountRepository{}
	first, _ := repository.Create(ctx, db, apiKeyAccount("One", accounts.ProviderAnthropic))
	repository.Create(ctx, db, apiKeyAccount("Two", accounts.ProviderOpenAI))

	if err := repository.UpdateName(ctx, db, first.ID, "Two", fixtureTime); !errors.Is(err, accounts.ErrAccountNameTaken) {
		t.Errorf("rename to a taken name error = %v", err)
	}
	if err := repository.UpdateName(ctx, db, first.ID, "Three", fixtureTime); err != nil {
		t.Errorf("rename error = %v", err)
	}
	if err := repository.UpdateName(ctx, db, 9999, "Four", fixtureTime); !errors.Is(err, accounts.ErrAccountNotFound) {
		t.Errorf("rename of a missing account error = %v", err)
	}
}

func TestCustomModelsAreUniquePerAccount(t *testing.T) {
	db := dbtest.NewMigratedSchema(t)
	ctx := context.Background()
	account, _ := accounts.AiAccountRepository{}.Create(ctx, db, apiKeyAccount("Claude", accounts.ProviderAnthropic))
	other, _ := accounts.AiAccountRepository{}.Create(ctx, db, apiKeyAccount("Claude 2", accounts.ProviderAnthropic))
	repository := accounts.AccountModelRepository{}

	if _, err := repository.Create(ctx, db, account.ID, "claude-next", "Next", fixtureTime); err != nil {
		t.Fatalf("create model: %v", err)
	}
	_, duplicate := repository.Create(ctx, db, account.ID, "claude-next", "Next", fixtureTime)
	if !errors.Is(duplicate, accounts.ErrModelAlreadyDefined) {
		t.Errorf("duplicate model error = %v", duplicate)
	}
	if _, err := repository.Create(ctx, db, other.ID, "claude-next", "Next", fixtureTime); err != nil {
		t.Errorf("same model on another account: %v", err)
	}

	byAccount, err := repository.ListByAccounts(ctx, db, []int64{account.ID, other.ID})
	if err != nil || len(byAccount[account.ID]) != 1 || len(byAccount[other.ID]) != 1 {
		t.Errorf("by account = %v, err = %v", byAccount, err)
	}
	if err := repository.SoftDeleteByToken(ctx, db, account.ID, byAccount[account.ID][0].Token, fixtureTime); err != nil {
		t.Errorf("remove model: %v", err)
	}
	if listed, _ := repository.ListByAccount(ctx, db, account.ID); len(listed) != 0 {
		t.Errorf("listed after removal = %v", listed)
	}
	if err := repository.SoftDeleteByToken(ctx, db, account.ID, "missing", fixtureTime); !errors.Is(err, accounts.ErrModelNotFound) {
		t.Errorf("remove missing model error = %v", err)
	}
}

func TestProviderCatalog(t *testing.T) {
	definitions := accounts.ProviderDefinitions()
	if len(definitions) != 4 {
		t.Fatalf("providers = %d", len(definitions))
	}
	xai, found := accounts.FindProviderDefinition(accounts.ProviderXAI)
	if !found || xai.Supports(accounts.AccountModeSubscription) || !xai.Supports(accounts.AccountModeAPIKey) || !xai.HasModel("grok-4") {
		t.Errorf("xai = %+v", xai)
	}
	if _, found := accounts.FindProviderDefinition("unknown"); found {
		t.Error("unknown provider found")
	}
	if accounts.LastCharactersOfKey("sk-abcdef1234") != "1234" || accounts.LastCharactersOfKey("ab") != "ab" {
		t.Error("LastCharactersOfKey is wrong")
	}
}
