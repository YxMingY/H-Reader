package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"hreader/llmkit"
	"hreader/services/config"

	openai "github.com/sashabaranov/go-openai"
)

func TestAPIProfileRoutingSurvivesRestartAndRestoresConversations(t *testing.T) {
	dir, configPath := isolatedAPIConfig(t)
	legacy := `{
		"default_folder":"papers",
		"api":{
			"profiles":[
				{"id":"reading","name":"Reading","provider":"aliyun","model":"reading-model","api_key":"legacy-key"},
				{"id":"quick","name":"Quick","provider":"aliyun","model":"quick-model","api_key":"legacy-key"},
				{"id":"duplicate","name":"Duplicate","provider":"aliyun","model":"reading-model","api_key":"legacy-key"},
				{"id":"other","name":"Other","provider":"deepseek","model":"other-model","api_key":"another-key"}
			],
			"sidebar_profile_id":"reading",
			"inline_profile_id":"quick"
		}
	}`
	if err := os.WriteFile(configPath, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	bookPath := filepath.Join(dir, "paper.pdf")
	if err := os.WriteFile(bookPath, []byte("example PDF"), 0600); err != nil {
		t.Fatal(err)
	}

	type modelRequest struct {
		key, path string
		openai.ChatCompletionRequest
	}
	requests := make(chan modelRequest, 10)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- modelRequest{key: r.Header.Get("Authorization"), path: r.URL.Path, ChatCompletionRequest: request}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"mock answer"}}]}`)
	}))
	defer api.Close()
	newService := func() *ChatService {
		t.Helper()
		cfg, err := config.LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.ReadDefaultFolder() != "papers" {
			t.Fatal("book folder lost")
		}
		return newChatService(cfg, func(cfg llmkit.Config) (*llmkit.Client, error) {
			if cfg.BaseURL == "" {
				if _, err := llmkit.NewClient(cfg); err != nil {
					return nil, err
				}
				cfg.BaseURL = api.URL + "/" + string(cfg.Provider)
			}
			return llmkit.NewClient(cfg)
		})
	}

	service := newService()
	settings := service.GetAPISettings()
	legacyID := settings.Selections.Sidebar.ProfileID
	if legacyID != "reading" || settings.Selections.Inline.ProfileID != legacyID || len(settings.Profiles) != 2 ||
		settings.Selections.Sidebar.Model != "reading-model" || settings.Selections.Inline.Model != "quick-model" ||
		!reflect.DeepEqual(settings.Profiles[0].Models, []string{"reading-model", "quick-model"}) {
		t.Fatalf("old models were not merged or restored: %+v", settings)
	}
	migrated, err := os.ReadFile(configPath)
	if err != nil || strings.Contains(string(migrated), `"sidebar_profile_id"`) {
		t.Fatalf("legacy format still on disk: %v", err)
	}
	view, err := json.Marshal(settings)
	if err != nil || strings.Contains(string(view), "legacy-key") || strings.Contains(string(view), "another-key") {
		t.Fatal("settings returned saved credentials")
	}

	sidebar, err := service.CreateSession(chatScopeBook, bookPath, "Sidebar history")
	if err != nil {
		t.Fatal(err)
	}
	inline, err := service.CreateInlineConversation(bookPath, InlineAnchor{PageNum: 1, X: 0.4, Y: 0.5, Quote: "paper excerpt"})
	if err != nil {
		t.Fatal(err)
	}
	assertRequest := func(model, key, firstQuestion, quote string) {
		t.Helper()
		req := <-requests
		if req.Model != model || req.key != "Bearer "+key {
			t.Fatalf("wrong model or credential for %q", model)
		}
		if firstQuestion != "" && (len(req.Messages) != 4 || req.Messages[1].Content != firstQuestion || req.Messages[2].Content != "mock answer") {
			t.Fatalf("missing restored conversation: %+v", req.Messages)
		}
		if quote != "" && !strings.Contains(req.Messages[0].Content, quote) {
			t.Fatal("lost PDF excerpt")
		}
	}
	sendBoth := func(service *ChatService, question, sidebarModel, sidebarKey, inlineModel, inlineKey string, continuing bool) {
		t.Helper()
		sidebarFirst, inlineFirst := "", ""
		if continuing {
			sidebarFirst, inlineFirst = "sidebar first", "inline first"
		}
		if _, err := service.SendMessageInSession(chatScopeBook, bookPath, sidebar.SessionID, "sidebar "+question, nil); err != nil {
			t.Fatal(err)
		}
		assertRequest(sidebarModel, sidebarKey, sidebarFirst, "")
		if _, err := service.SendInlineQuestion(context.Background(), bookPath, inline.ID, "inline "+question); err != nil {
			t.Fatal(err)
		}
		assertRequest(inlineModel, inlineKey, inlineFirst, "paper excerpt")
	}

	sendBoth(service, "first", "reading-model", "legacy-key", "quick-model", "legacy-key", false)
	updated, err := service.SaveAPIProfile(APIProfileInput{
		ID: legacyID, Name: "Reading", Provider: "aliyun",
		Models: []string{"quick-model", "new-reading-model", " new-fast-model ", "new-fast-model"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Selections.Sidebar != (config.APISelection{ProfileID: legacyID}) ||
		updated.Selections.Inline != settings.Selections.Inline ||
		!reflect.DeepEqual(updated.Profiles[0].Models, []string{"quick-model", "new-reading-model", "new-fast-model"}) {
		t.Fatalf("removing the selected model affected the wrong settings: %+v", updated)
	}
	if _, err := service.SaveAPIProfile(APIProfileInput{Name: "Duplicate", Provider: "aliyun", APIKey: "other"}); err == nil {
		t.Fatal("supplier already configured")
	}
	if _, err := service.SaveAPISelections(APISelections{Sidebar: config.APISelection{ProfileID: legacyID, Model: "reading-model"}}); err == nil {
		t.Fatal("selection accepted a removed model")
	}
	wanted := APISelections{
		Sidebar: config.APISelection{ProfileID: legacyID, Model: "new-reading-model"},
		Inline:  config.APISelection{ProfileID: legacyID, Model: "new-fast-model"},
	}
	saved, err := service.SaveAPISelections(wanted)
	if err != nil {
		t.Fatal(err)
	}
	service = newService()
	if got := service.GetAPISettings(); !reflect.DeepEqual(got, saved) {
		t.Fatalf("API settings changed on restart: %+v", got)
	}
	sendBoth(service, "follow-up", "new-reading-model", "legacy-key", "new-fast-model", "legacy-key", true)

	custom, err := service.SaveAPIProfile(APIProfileInput{
		Name: "Custom endpoint", Provider: "custom", Models: []string{"custom-model"},
		BaseURL: api.URL + "/custom/v1/chat/completions", APIKey: "custom-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	profile := custom.Profiles[len(custom.Profiles)-1]
	if profile.BaseURL != api.URL+"/custom/v1" {
		t.Fatalf("endpoint not normalized: %q", profile.BaseURL)
	}
	if _, err := service.SaveAPIProfile(APIProfileInput{
		ID: profile.ID, Name: profile.Name, Provider: "custom", Models: profile.Models, BaseURL: api.URL + "/different",
	}); err == nil {
		t.Fatal("an edited endpoint accepted the previous key")
	}
	wanted.Inline = config.APISelection{ProfileID: profile.ID, Model: "custom-model"}
	if _, err := service.SaveAPISelections(wanted); err != nil {
		t.Fatal(err)
	}
	service = newService()
	if got := service.GetAPISettings().Selections; got != wanted {
		t.Fatalf("custom endpoint selection changed on restart: %+v", got)
	}
	if _, err := service.SendInlineQuestion(context.Background(), bookPath, inline.ID, "using custom endpoint"); err != nil {
		t.Fatal(err)
	}
	req := <-requests
	if req.path != "/custom/v1/chat/completions" || req.Model != "custom-model" || req.key != "Bearer custom-key" ||
		len(req.Messages) != 6 || req.Messages[3].Content != "inline follow-up" {
		t.Fatalf("custom endpoint did not receive the restored conversation; path=%s model=%s", req.path, req.Model)
	}
}

func TestLegacySingleAPIKeyMigratesToBothEntrances(t *testing.T) {
	_, path := isolatedAPIConfig(t)
	legacy := `{"api_key":"single-key","api_keys":{},"provider":"aliyun","model":"custom-qwen"}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	settings := cfg.ReadAPISettings()
	profile, exists := settings.Profile(settings.Sidebar.ProfileID)
	if !exists || settings.Inline != settings.Sidebar || settings.Sidebar.Model != "custom-qwen" || profile.APIKey != "single-key" || !reflect.DeepEqual(profile.Models, []string{"custom-qwen"}) {
		t.Fatalf("single API key not migrated: selected profile exists = %v", exists)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(updated, &fields); err != nil {
		t.Fatal(err)
	}
	for _, obsolete := range []string{"api_key", "api_keys", "provider", "model"} {
		if _, found := fields[obsolete]; found {
			t.Fatalf("legacy field %q was left in config.json", obsolete)
		}
	}
	var persisted config.APISettings
	if err := json.Unmarshal(fields["api"], &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.Version != 2 || !reflect.DeepEqual(settings, persisted) {
		t.Fatal("migrated key or model choices were not written to config.json")
	}
	reopened, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reopened.ReadAPISettings(), persisted) {
		t.Fatal("reopened config did not restore the migrated key and model choices")
	}

}

// Verify the platform's configuration resolution before reading or writing fixtures.
func isolatedAPIConfig(t *testing.T) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Chdir(dir)

	path = filepath.Join(dir, "config.json")
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		expected := dir
		if runtime.GOOS == "darwin" {
			expected = filepath.Join(dir, "Library", "Application Support")
		}
		resolved, err := os.UserConfigDir()
		if err != nil || resolved != expected {
			t.Fatalf("refusing to use a non-test config directory: %q, %v", resolved, err)
		}
		path = filepath.Join(expected, "hreader", "config.json")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

func TestMigrationPreservesSelectedModelWithoutKey(t *testing.T) {
	_, path := isolatedAPIConfig(t)
	legacy := `{"api_keys":{"deepseek":"other-key"},"provider":"aliyun","model":"saved-custom-model"}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	settings := cfg.ReadAPISettings()
	if settings.Sidebar.ProfileID != "" || settings.Inline.ProfileID != "" {
		t.Fatal("migration activated a different provider")
	}
	profile, exists := settings.Profile("legacy-aliyun")
	if !exists || !reflect.DeepEqual(profile.Models, []string{"saved-custom-model"}) || profile.APIKey != "" {
		t.Fatal("migration lost the model selected before a key was configured")
	}
}
