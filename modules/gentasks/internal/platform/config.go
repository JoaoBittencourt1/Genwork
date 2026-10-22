package platform

import (
	"encoding/base64"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/envconfig"
)

type Getenv = envconfig.Getenv

const Schema = "gentasks"

// MinEncryptionKeyBytes is the 256 bits the installer generates for GENTASKS_ENCRYPTION_KEY.
const MinEncryptionKeyBytes = 32

type ServeConfig struct {
	DatabaseURL     string
	HTTPAddr        string
	CoreInternalURL string
	ModuleSecret    string
	PublicURL       string
	EncryptionKey   []byte
	GithubWebURL    string
	GithubAPIURL    string
}

// readEncryptionKey never echoes the value in its error, so a weak key does not end up in logs.
func readEncryptionKey(reader *envconfig.EnvReader, name string) []byte {
	value := reader.Required(name)
	if value == "" {
		return nil
	}
	key, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(key) < MinEncryptionKeyBytes {
		reader.MarkInvalid(name + " (at least 32 bytes as unpadded base64url)")
		return nil
	}
	return key
}

func LoadDatabaseURL(getenv Getenv) (string, error) {
	reader := envconfig.NewEnvReader(getenv)
	databaseURL := reader.Required("GENTASKS_DATABASE_URL")
	return databaseURL, reader.Err()
}

func LoadServeConfig(getenv Getenv) (ServeConfig, error) {
	reader := envconfig.NewEnvReader(getenv)
	config := ServeConfig{
		DatabaseURL:     reader.Required("GENTASKS_DATABASE_URL"),
		HTTPAddr:        reader.Required("GENTASKS_HTTP_ADDR"),
		CoreInternalURL: reader.RequiredURL("CORE_INTERNAL_URL"),
		ModuleSecret:    reader.RequiredModuleSecret("GENWORK_MODULE_SECRET"),
		PublicURL:       reader.RequiredURL("GENWORK_PUBLIC_URL"),
		EncryptionKey:   readEncryptionKey(reader, "GENTASKS_ENCRYPTION_KEY"),
		GithubWebURL:    reader.RequiredURL("GITHUB_WEB_URL"),
		GithubAPIURL:    reader.RequiredURL("GITHUB_API_URL"),
	}
	return config, reader.Err()
}
