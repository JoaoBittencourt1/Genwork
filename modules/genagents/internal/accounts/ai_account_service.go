package accounts

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/runners"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const apiKeyColumn = "ai_accounts.api_key"

// AccountUsage names what keeps an account from being removed; bots implement it (rule 9).
type AccountUsage interface {
	ListBotNamesUsingAccount(ctx context.Context, db platform.DBTX, accountID int64) ([]string, error)
}

// AccountWithModels is an account with its provider catalog models and the custom models added to it.
type AccountWithModels struct {
	Account      AiAccount
	CustomModels []AccountModel
	Runner       *runners.Runner
}

type AiAccountService struct {
	db      *pgxpool.Pool
	cipher  *platform.SecretCipher
	usage   AccountUsage
	runners runners.RunnerRepository
	repo    AiAccountRepository
	models  AccountModelRepository
	now     func() time.Time
}

func NewAiAccountService(db *pgxpool.Pool, cipher *platform.SecretCipher, usage AccountUsage, now func() time.Time) *AiAccountService {
	return &AiAccountService{db: db, cipher: cipher, usage: usage, now: now}
}

func AssociatedDataForAPIKey(accountToken string) string {
	return apiKeyColumn + "|" + accountToken
}

func (service *AiAccountService) ListAccounts(ctx context.Context) ([]AccountWithModels, error) {
	listed, err := service.repo.List(ctx, service.db)
	if err != nil {
		return nil, err
	}
	return service.attachDetails(ctx, service.db, listed)
}

func (service *AiAccountService) FindAccount(ctx context.Context, token string) (AccountWithModels, error) {
	account, err := service.repo.FindByToken(ctx, service.db, token)
	if err != nil {
		return AccountWithModels{}, err
	}
	detailed, err := service.attachDetails(ctx, service.db, []AiAccount{account})
	if err != nil {
		return AccountWithModels{}, err
	}
	return detailed[0], nil
}

func (service *AiAccountService) CreateAccount(ctx context.Context, input CreateAiAccountInput) (AccountWithModels, error) {
	token := platform.NewPublicToken()
	newAccount := NewAiAccount{Token: token, Name: input.Name, Provider: input.Provider, Mode: input.Mode, At: service.now()}
	if input.Mode == AccountModeAPIKey {
		ciphertext, err := service.cipher.Encrypt(input.APIKey, AssociatedDataForAPIKey(token))
		if err != nil {
			return AccountWithModels{}, err
		}
		last4 := LastCharactersOfKey(input.APIKey)
		newAccount.APIKeyCiphertext, newAccount.APIKeyLast4, newAccount.Status = ciphertext, &last4, AccountStatusReady
	}
	var created AiAccount
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		if input.Mode == AccountModeSubscription {
			runnerID, err := service.findRunnerReportingCLI(ctx, tx, input)
			if err != nil {
				return err
			}
			newAccount.RunnerID, newAccount.Status = &runnerID, AccountStatusPendingSignIn
		}
		var err error
		created, err = service.repo.Create(ctx, tx, newAccount)
		return err
	})
	if err != nil {
		return AccountWithModels{}, err
	}
	return service.FindAccount(ctx, created.Token)
}

func (service *AiAccountService) findRunnerReportingCLI(ctx context.Context, tx pgx.Tx, input CreateAiAccountInput) (int64, error) {
	runner, err := service.runners.FindByToken(ctx, tx, input.RunnerToken)
	if errors.Is(err, runners.ErrRunnerNotFound) || (err == nil && runner.IsRevoked()) {
		return 0, httpapi.NewValidationError(map[string]string{"runnerToken": "unknown"})
	}
	if err != nil {
		return 0, err
	}
	definition, _ := FindProviderDefinition(input.Provider)
	if !runner.ReportsTool(definition.CLI) {
		return 0, httpapi.NewValidationError(map[string]string{"runnerToken": "missing_provider_cli"})
	}
	return runner.ID, nil
}

func (service *AiAccountService) UpdateAccount(ctx context.Context, token string, changes AiAccountChanges) (AccountWithModels, error) {
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		account, err := service.repo.FindByToken(ctx, tx, token)
		if err != nil {
			return err
		}
		now := service.now()
		if changes.Name != nil {
			if err := service.repo.UpdateName(ctx, tx, account.ID, *changes.Name, now); err != nil {
				return err
			}
		}
		if changes.APIKey != nil {
			if account.Mode != AccountModeAPIKey {
				return httpapi.NewValidationError(map[string]string{"apiKey": "not_applicable"})
			}
			ciphertext, err := service.cipher.Encrypt(*changes.APIKey, AssociatedDataForAPIKey(account.Token))
			if err != nil {
				return err
			}
			return service.repo.ReplaceAPIKey(ctx, tx, account.ID, ciphertext, LastCharactersOfKey(*changes.APIKey), now)
		}
		return nil
	})
	if err != nil {
		return AccountWithModels{}, err
	}
	return service.FindAccount(ctx, token)
}

// ErrAccountInUse lists the bots that use the account, so the admin knows what to change first.
func newAccountInUseError(botNames []string) *httpapi.APIError {
	return &httpapi.APIError{Status: http.StatusConflict, Code: "account_in_use", Fields: map[string]string{"bots": strings.Join(botNames, ", ")}}
}

func (service *AiAccountService) RemoveAccount(ctx context.Context, token string) error {
	return pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		account, err := service.repo.FindByToken(ctx, tx, token)
		if err != nil {
			return err
		}
		botNames, err := service.usage.ListBotNamesUsingAccount(ctx, tx, account.ID)
		if err != nil {
			return err
		}
		if len(botNames) > 0 {
			return newAccountInUseError(botNames)
		}
		return service.repo.SoftDelete(ctx, tx, account.ID, service.now())
	})
}

func (service *AiAccountService) AddModel(ctx context.Context, accountToken string, input AddAccountModelInput) (AccountWithModels, error) {
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		account, err := service.repo.FindByToken(ctx, tx, accountToken)
		if err != nil {
			return err
		}
		definition, _ := FindProviderDefinition(account.Provider)
		if definition.HasModel(input.ModelID) {
			return ErrModelAlreadyDefined
		}
		_, err = service.models.Create(ctx, tx, account.ID, input.ModelID, input.DisplayName, service.now())
		return err
	})
	if err != nil {
		return AccountWithModels{}, err
	}
	return service.FindAccount(ctx, accountToken)
}

func (service *AiAccountService) RemoveModel(ctx context.Context, accountToken, modelToken string) error {
	account, err := service.repo.FindByToken(ctx, service.db, accountToken)
	if err != nil {
		return err
	}
	return service.models.SoftDeleteByToken(ctx, service.db, account.ID, modelToken, service.now())
}

// DecryptAPIKey is the only way to read a key back; run claiming uses it to hand the key to a runner for one run.
func (service *AiAccountService) DecryptAPIKey(account AiAccount) (string, error) {
	return service.cipher.Decrypt(account.APIKeyCiphertext, AssociatedDataForAPIKey(account.Token))
}

// ModelIsAvailable reports whether a bot may use the model on the account: its provider catalog or its custom models.
func (service *AiAccountService) ModelIsAvailable(ctx context.Context, db platform.DBTX, account AiAccount, modelID string) (bool, error) {
	definition, _ := FindProviderDefinition(account.Provider)
	if definition.HasModel(modelID) {
		return true, nil
	}
	custom, err := service.models.ListByAccount(ctx, db, account.ID)
	if err != nil {
		return false, err
	}
	for _, model := range custom {
		if model.ModelID == modelID {
			return true, nil
		}
	}
	return false, nil
}

func (service *AiAccountService) attachDetails(ctx context.Context, db platform.DBTX, listed []AiAccount) ([]AccountWithModels, error) {
	ids := make([]int64, 0, len(listed))
	for _, account := range listed {
		ids = append(ids, account.ID)
	}
	customByAccount, err := service.models.ListByAccounts(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	allRunners, err := service.runners.List(ctx, db)
	if err != nil {
		return nil, err
	}
	runnersByID := make(map[int64]runners.Runner, len(allRunners))
	for _, runner := range allRunners {
		runnersByID[runner.ID] = runner
	}
	detailed := make([]AccountWithModels, 0, len(listed))
	for _, account := range listed {
		entry := AccountWithModels{Account: account, CustomModels: customByAccount[account.ID]}
		if account.RunnerID != nil {
			if runner, found := runnersByID[*account.RunnerID]; found {
				entry.Runner = &runner
			}
		}
		detailed = append(detailed, entry)
	}
	return detailed, nil
}
