package command

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/users"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminCreateOptions struct {
	Email         string
	Name          string
	PasswordStdin bool
}

func ParseAdminCreateOptions(args []string) (AdminCreateOptions, error) {
	var options AdminCreateOptions
	flags := flag.NewFlagSet("admin create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.Email, "email", "", "administrator e-mail")
	flags.StringVar(&options.Name, "name", "", "administrator name")
	flags.BoolVar(&options.PasswordStdin, "password-stdin", false, "read the password from standard input")
	if err := flags.Parse(args); err != nil {
		return AdminCreateOptions{}, err
	}
	if strings.TrimSpace(options.Email) == "" || strings.TrimSpace(options.Name) == "" || !options.PasswordStdin {
		return AdminCreateOptions{}, errors.New("usage: core admin create --email <email> --name <name> --password-stdin")
	}
	return options, nil
}

// ReadPasswordLine keeps the password out of arguments and environment, where other processes could read it.
func ReadPasswordLine(stdin io.Reader) (string, error) {
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read password: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func RunAdminCreate(ctx context.Context, getenv platform.Getenv, schema string, args []string, stdin io.Reader, stdout io.Writer) error {
	options, err := ParseAdminCreateOptions(args)
	if err != nil {
		return err
	}
	password, err := ReadPasswordLine(stdin)
	if err != nil {
		return err
	}
	databaseURL, err := platform.LoadDatabaseURL(getenv)
	if err != nil {
		return err
	}
	db, err := platform.OpenDatabase(ctx, databaseURL, schema)
	if err != nil {
		return err
	}
	defer db.Close()

	user, err := CreateAdministrator(ctx, db, users.DefaultPasswordParams, options.Email, options.Name, password)
	if err != nil {
		return DescribeAdminCreateError(err)
	}
	_, err = fmt.Fprintf(stdout, "administrator created: %s (%s)\n", user.Email, user.Token)
	return err
}

var adminCreateMessages = map[string]string{
	"password_length_invalid":  fmt.Sprintf("the password must be between %d and %d characters", users.MinPasswordLength, users.MaxPasswordLength),
	"email_already_registered": "a user with this e-mail already exists",
}

// DescribeAdminCreateError keeps the machine-readable code and adds a sentence for the operator reading the terminal.
func DescribeAdminCreateError(err error) error {
	var apiErr *platform.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	if message, found := adminCreateMessages[apiErr.Code]; found {
		return fmt.Errorf("%s: %s", apiErr.Code, message)
	}
	if len(apiErr.Fields) > 0 {
		var invalid []string
		for field := range apiErr.Fields {
			invalid = append(invalid, field)
		}
		slices.Sort(invalid)
		return fmt.Errorf("%s: invalid %s", apiErr.Code, strings.Join(invalid, ", "))
	}
	return apiErr
}

func CreateAdministrator(ctx context.Context, db *pgxpool.Pool, params users.PasswordParams, email, name, password string) (users.User, error) {
	adminRole, err := access.RoleRepository{}.FindByName(ctx, db, access.RoleNameAdmin)
	if err != nil {
		return users.User{}, err
	}
	service := users.NewService(users.NewPasswordHasher(params, 1))
	return service.Create(ctx, db, users.CreateInput{Email: email, Name: name, Password: password, RoleID: adminRole.ID})
}
