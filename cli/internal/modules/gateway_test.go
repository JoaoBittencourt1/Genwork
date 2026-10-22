package modules

import (
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/manifest"
)

func TestConventionsFollowTheModuleID(t *testing.T) {
	conventions := ConventionsFor("time-tracking")
	if conventions.Profile != "time-tracking" || conventions.MigrateService != "time-tracking-migrate" || conventions.DatabaseRole != "time-tracking" ||
		conventions.PasswordVariable != "TIME_TRACKING_DB_PASSWORD" || conventions.DatabaseURLVariable != "TIME_TRACKING_DATABASE_URL" ||
		conventions.UpstreamVariable != "TIME_TRACKING_UPSTREAM" || conventions.EncryptionKeyVariable != "TIME_TRACKING_ENCRYPTION_KEY" {
		t.Errorf("conventions = %+v", conventions)
	}
	if services := ConventionsFor("gentasks").Services(); len(services) != 2 || services[0] != "gentasks-migrate" || services[1] != "gentasks" {
		t.Errorf("services = %v", services)
	}
}

func TestRenderCaddyfileRoutesCoreAndInstalledModules(t *testing.T) {
	rendered, err := RenderCaddyfile([]manifest.ModuleManifest{moduleFixture("wiki", "1.0.0"), moduleFixture("gentasks", "0.1.0")})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	content := string(rendered)

	for _, expected := range []string{"handle /internal/* {\n\t\trespond 404", "handle /api/core/* {", "handle /.well-known/* {", "reverse_proxy {$CORE_UPSTREAM}"} {
		if !strings.Contains(content, expected) {
			t.Errorf("missing %q in:\n%s", expected, content)
		}
	}
	gentasks := strings.Index(content, "handle /api/gentasks/v1/* {\n\t\treverse_proxy {$GENTASKS_UPSTREAM}")
	wiki := strings.Index(content, "handle /api/wiki/v1/* {\n\t\treverse_proxy {$WIKI_UPSTREAM}")
	if gentasks < 0 || wiki < 0 || gentasks > wiki {
		t.Errorf("module routes missing or unsorted:\n%s", content)
	}
}

func TestRenderCaddyfileWithoutModules(t *testing.T) {
	rendered, err := RenderCaddyfile(nil)
	if err != nil || strings.Contains(string(rendered), "UPSTREAM}\n\t}\n\n\thandle /api/gentasks") || !strings.Contains(string(rendered), "handle /api/core/*") {
		t.Errorf("rendered = %s, err = %v", rendered, err)
	}
	if strings.Contains(string(rendered), "gentasks") {
		t.Errorf("no module route expected:\n%s", rendered)
	}
}

func TestRenderCaddyfileRoutesTheMCPServer(t *testing.T) {
	rendered, err := RenderCaddyfile(nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	content := string(rendered)

	for _, expected := range []string{
		"handle /mcp {\n\t\treverse_proxy {$MCP_UPSTREAM}",
		"handle /mcp/* {\n\t\treverse_proxy {$MCP_UPSTREAM}",
	} {
		if !strings.Contains(content, expected) {
			t.Errorf("missing %q in:\n%s", expected, content)
		}
	}
	resourceMetadata := strings.Index(content, "handle /.well-known/oauth-protected-resource* {\n\t\treverse_proxy {$MCP_UPSTREAM}")
	wellKnown := strings.Index(content, "handle /.well-known/* {")
	if resourceMetadata < 0 || resourceMetadata > wellKnown {
		t.Errorf("the protected resource metadata must be routed to the MCP server before /.well-known/*:\n%s", content)
	}
}

func TestRenderCaddyfileServesTheWebAppOnEveryOtherPath(t *testing.T) {
	rendered, err := RenderCaddyfile([]manifest.ModuleManifest{moduleFixture("gentasks", "0.5.0")})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	content := string(rendered)

	catchAll := strings.LastIndex(content, "\thandle {\n")
	if catchAll < 0 || catchAll < strings.Index(content, "handle /api/gentasks/v1/*") {
		t.Fatalf("the web app must be the last route:\n%s", content)
	}
	for _, expected := range []string{"root * /srv/web", "header Cache-Control \"no-cache\"", "try_files {path} /index.html", "file_server"} {
		if !strings.Contains(content[catchAll:], expected) {
			t.Errorf("web app route is missing %q:\n%s", expected, content[catchAll:])
		}
	}
	assets := "handle /assets/* {\n\t\troot * /srv/web\n\t\t@existingAsset file\n\t\theader @existingAsset Cache-Control \"public, max-age=31536000, immutable\"\n\t\tfile_server\n\t}"
	if !strings.Contains(content, assets) {
		t.Errorf("hashed assets must be immutable and never fall back to index.html:\n%s", content)
	}
	if !strings.Contains(content, "handle /api/* {\n\t\trespond 404") {
		t.Errorf("unknown API paths must answer 404 instead of the web app:\n%s", content)
	}
}
