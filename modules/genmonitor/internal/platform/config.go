package platform

import "github.com/JoaoBittencourt1/Genwork/sdk/go/envconfig"

type Getenv = envconfig.Getenv

type ServeConfig struct {
	HTTPAddr        string
	CoreInternalURL string
	ModuleSecret    string
	PublicURL       string
	HostProcPath    string
	HostRootPath    string
}

func LoadServeConfig(getenv Getenv) (ServeConfig, error) {
	reader := envconfig.NewEnvReader(getenv)
	config := ServeConfig{
		HTTPAddr:        reader.Required("GENMONITOR_HTTP_ADDR"),
		CoreInternalURL: reader.RequiredURL("CORE_INTERNAL_URL"),
		ModuleSecret:    reader.RequiredModuleSecret("GENWORK_MODULE_SECRET"),
		PublicURL:       reader.RequiredURL("GENWORK_PUBLIC_URL"),
		HostProcPath:    reader.Required("GENMONITOR_HOST_PROC"),
		HostRootPath:    reader.Required("GENMONITOR_HOST_ROOT"),
	}
	return config, reader.Err()
}
