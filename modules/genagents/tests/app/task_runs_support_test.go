package app_test

import (
	"encoding/json"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/app"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/corelink"
	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/gentasksclient"
	"time"
)

func jsonUnmarshal(data []byte, target any) error {
	return json.Unmarshal(data, target)
}

// appWorkers builds the module's background workers over the same database and stubs the HTTP app uses.
func appWorkers(testApp *testApp) ([]app.Worker, error) {
	return app.NewWorkers(app.Dependencies{
		DB:            testApp.db,
		EncryptionKey: testEncryptionKey,
		Core:          corelink.NewClient(testApp.coreAPI.URL(), time.Second),
		Gentasks:      gentasksclient.NewClient(testApp.tasks.URL(), time.Second),
		Now:           testApp.clock.Now,
	})
}
