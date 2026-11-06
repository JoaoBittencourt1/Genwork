package app_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
)

type skillBody struct {
	Token       string `json:"token"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
	Version     int    `json:"version"`
	Files       []struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	} `json:"files"`
}

func (app *testApp) createSkill(body map[string]any) response {
	return app.do(request{method: "POST", path: "/api/genagents/v1/skills", body: body})
}

func (app *testApp) createGoSkill() skillBody {
	app.t.Helper()
	resp := app.createSkill(map[string]any{
		"name": "go-conventions", "description": "How we write Go", "body": "# Go\nUse small functions.",
		"files": []map[string]any{{"path": "examples/handler.go", "content": "package x"}},
	})
	expectStatus(app.t, resp, http.StatusCreated)
	var skill skillBody
	resp.decode(app.t, &skill)
	return skill
}

func TestCreateAndReadSkill(t *testing.T) {
	app := newTestApp(t)

	skill := app.createGoSkill()

	if skill.Version != 1 || skill.Name != "go-conventions" || len(skill.Files) != 1 || skill.Files[0].Path != "examples/handler.go" {
		t.Fatalf("skill = %+v", skill)
	}
	resp := app.do(request{method: "GET", path: "/api/genagents/v1/skills/" + skill.Token})
	expectStatus(t, resp, http.StatusOK)
	list := app.do(request{method: "GET", path: "/api/genagents/v1/skills"})
	if !strings.Contains(string(list.body), "go-conventions") {
		t.Errorf("list = %s", list.body)
	}
}

func TestSkillValidation(t *testing.T) {
	app := newTestApp(t)
	valid := func() map[string]any {
		return map[string]any{"name": "ok-name", "description": "d", "body": "b"}
	}
	cases := map[string]func(map[string]any){
		"name":        func(body map[string]any) { body["name"] = "Bad Name" },
		"description": func(body map[string]any) { body["description"] = "" },
		"body":        func(body map[string]any) { body["body"] = "  " },
		"files": func(body map[string]any) {
			body["files"] = []map[string]any{{"path": "../../etc/passwd", "content": "x"}}
		},
	}
	for field, mutate := range cases {
		t.Run(field, func(t *testing.T) {
			body := valid()
			mutate(body)
			expectValidationError(t, app.createSkill(body), field)
		})
	}
	longName := valid()
	longName["name"] = strings.Repeat("a", 65)
	expectValidationError(t, app.createSkill(longName), "name")
}

func TestSkillFilePathRules(t *testing.T) {
	app := newTestApp(t)
	for _, path := range []string{"/abs/path", "a/../b", "a//b", "./a", "SKILL.md", "a\\b", ""} {
		body := map[string]any{"name": "ok", "description": "d", "body": "b", "files": []map[string]any{{"path": path, "content": "x"}}}
		expectValidationError(t, app.createSkill(body), "files")
	}
	duplicates := map[string]any{"name": "ok", "description": "d", "body": "b", "files": []map[string]any{{"path": "a", "content": "x"}, {"path": "a", "content": "y"}}}
	expectValidationError(t, app.createSkill(duplicates), "files")
	tooMany := make([]map[string]any, 21)
	for index := range tooMany {
		tooMany[index] = map[string]any{"path": strings.Repeat("f", index+1), "content": "x"}
	}
	expectValidationError(t, app.createSkill(map[string]any{"name": "ok", "description": "d", "body": "b", "files": tooMany}), "files")
	large := map[string]any{"name": "ok", "description": "d", "body": "b", "files": []map[string]any{{"path": "big", "content": strings.Repeat("x", 256*1024+1)}}}
	expectValidationError(t, app.createSkill(large), "files")
}

func TestSkillNamesAreUnique(t *testing.T) {
	app := newTestApp(t)
	app.createGoSkill()

	resp := app.createSkill(map[string]any{"name": "go-conventions", "description": "d", "body": "b"})

	expectError(t, resp, http.StatusConflict, "skill_name_taken")
}

func TestImportSkillFromSkillMarkdown(t *testing.T) {
	app := newTestApp(t)
	content := "---\nname: pr-review\ndescription: \"Reviews pull requests\"\n---\n# Steps\nRead the diff.\n"

	resp := app.do(request{method: "POST", path: "/api/genagents/v1/skills/import", body: map[string]any{"content": content}})

	expectStatus(t, resp, http.StatusCreated)
	var skill skillBody
	resp.decode(t, &skill)
	if skill.Name != "pr-review" || skill.Description != "Reviews pull requests" || !strings.HasPrefix(skill.Body, "# Steps") {
		t.Errorf("skill = %+v", skill)
	}
	expectValidationError(t, app.do(request{method: "POST", path: "/api/genagents/v1/skills/import", body: map[string]any{"content": "no front matter"}}), "content")
	expectValidationError(t, app.do(request{method: "POST", path: "/api/genagents/v1/skills/import", body: map[string]any{"content": "---\nname: x\n---\nbody"}}), "content")
}

func TestEditingASkillBumpsItsVersionAndSingleFieldPatchKeepsTheRest(t *testing.T) {
	app := newTestApp(t)
	skill := app.createGoSkill()
	path := "/api/genagents/v1/skills/" + skill.Token

	resp := app.do(request{method: "PATCH", path: path, body: map[string]any{"description": "Updated"}})

	expectStatus(t, resp, http.StatusOK)
	var updated skillBody
	resp.decode(t, &updated)
	if updated.Version != 2 || updated.Description != "Updated" || updated.Name != skill.Name || updated.Body != skill.Body || len(updated.Files) != 1 {
		t.Errorf("updated = %+v", updated)
	}
	replaced := app.do(request{method: "PATCH", path: path, body: map[string]any{"files": []map[string]any{}}})
	var withoutFiles skillBody
	replaced.decode(t, &withoutFiles)
	if withoutFiles.Version != 3 || len(withoutFiles.Files) != 0 || withoutFiles.Description != "Updated" {
		t.Errorf("without files = %+v", withoutFiles)
	}
	expectValidationError(t, app.do(request{method: "PATCH", path: path, body: map[string]any{"body": nil}}), "body")
	expectValidationError(t, app.do(request{method: "PATCH", path: path, body: map[string]any{"name": "BAD"}}), "name")
	expectError(t, app.do(request{method: "PATCH", path: "/api/genagents/v1/skills/missing", body: map[string]any{"name": "x"}}), http.StatusNotFound, "skill_not_found")
}

func TestDeleteSkill(t *testing.T) {
	app := newTestApp(t)
	skill := app.createGoSkill()

	expectStatus(t, app.do(request{method: "DELETE", path: "/api/genagents/v1/skills/" + skill.Token}), http.StatusNoContent)

	expectError(t, app.do(request{method: "GET", path: "/api/genagents/v1/skills/" + skill.Token}), http.StatusNotFound, "skill_not_found")
	if resp := app.createSkill(map[string]any{"name": "go-conventions", "description": "d", "body": "b"}); resp.status != http.StatusCreated {
		t.Errorf("reusing the name of a deleted skill: %s", resp.body)
	}
}

func TestSkillManagementNeedsThePermission(t *testing.T) {
	app := newTestApp(t)
	member := app.tokenWith(access.PermissionBotRead)

	for _, call := range []request{
		{method: "GET", path: "/api/genagents/v1/skills"},
		{method: "POST", path: "/api/genagents/v1/skills", body: map[string]any{}},
		{method: "POST", path: "/api/genagents/v1/skills/import", body: map[string]any{}},
		{method: "GET", path: "/api/genagents/v1/skills/x"},
		{method: "PATCH", path: "/api/genagents/v1/skills/x", body: map[string]any{}},
		{method: "DELETE", path: "/api/genagents/v1/skills/x"},
	} {
		call.accessToken = member
		expectError(t, app.do(call), http.StatusForbidden, "forbidden")
	}
}
