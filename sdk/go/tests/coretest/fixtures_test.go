package coretest_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/coreclient"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/coretest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/registration"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/tokens"
)

type errorFixture struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

// The SDK types must decode the recorded core responses with nothing left over, so they stay in step with core.
func TestSDKTypesMatchRecordedFixtures(t *testing.T) {
	targets := map[string]any{
		"jwks.json":                      &tokens.JSONWebKeySet{},
		"directory_user.json":            &coreclient.DirectoryUser{},
		"user_not_found.json":            &errorFixture{},
		"registered_module.json":         &registration.RegisteredModule{},
		"incompatible_core_version.json": &errorFixture{},
		"capabilities.json":              &map[string]any{},
	}
	for _, name := range coretest.FixtureNames {
		t.Run(name, func(t *testing.T) {
			target, found := targets[name]
			if !found {
				t.Fatalf("no SDK type covers fixture %s", name)
			}
			decoder := json.NewDecoder(bytes.NewReader(coretest.Fixture(name)))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(target); err != nil {
				t.Errorf("decode %s: %v", name, err)
			}
		})
	}
}

func TestFixturePanicsOnUnknownName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic")
		}
	}()
	coretest.Fixture("missing.json")
}
