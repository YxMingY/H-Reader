package chat

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"hreader/llmkit"
	"hreader/services/config"
)

type APIProfileInput struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Provider string   `json:"provider"`
	BaseURL  string   `json:"baseUrl"`
	Models   []string `json:"models"`
	APIKey   string   `json:"apiKey"`
}

type APIProfileView struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Provider string   `json:"provider"`
	BaseURL  string   `json:"baseUrl"`
	Models   []string `json:"models"`
	HasKey   bool     `json:"hasKey"`
}

type APISelections struct {
	Sidebar config.APISelection `json:"sidebar"`
	Inline  config.APISelection `json:"inline"`
}

type APISettingsView struct {
	Profiles   []APIProfileView `json:"profiles"`
	Selections APISelections    `json:"selections"`
}

func (c *ChatService) GetAPISettings() APISettingsView {
	return apiSettingsView(c.config.ReadAPISettings())
}

func (c *ChatService) SaveAPIProfile(input APIProfileInput) (APISettingsView, error) {
	profile := config.APIProfile{
		ID: input.ID, Name: strings.TrimSpace(input.Name), Provider: strings.TrimSpace(input.Provider),
		APIKey: strings.TrimSpace(input.APIKey), Models: []string{},
	}
	if profile.Name == "" || profile.Provider == "" {
		return APISettingsView{}, errors.New("请填写配置名称和供应商")
	}
	if profile.Provider == "custom" {
		var err error
		profile.BaseURL, err = customAPIBaseURL(input.BaseURL)
		if err != nil {
			return APISettingsView{}, err
		}
	}
	for _, item := range input.Models {
		model := strings.TrimSpace(item)
		if model != "" && !slices.Contains(profile.Models, model) {
			profile.Models = append(profile.Models, model)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.config.UpdateAPISettings(func(settings *config.APISettings) error {
		index := slices.IndexFunc(settings.Profiles, func(old config.APIProfile) bool {
			return old.ID == profile.ID
		})
		if profile.ID != "" && index < 0 {
			return errors.New("此供应商配置已不存在，请重新打开设置")
		}
		sameEndpoint := index >= 0 && sameSupplier(settings.Profiles[index], profile)
		if !sameEndpoint && slices.ContainsFunc(settings.Profiles, func(old config.APIProfile) bool {
			return sameSupplier(old, profile)
		}) {
			return errors.New("此供应商已配置，请编辑现有配置的模型列表")
		}
		if profile.APIKey == "" && sameEndpoint {
			profile.APIKey = settings.Profiles[index].APIKey
		}
		if profile.APIKey == "" {
			return errors.New("请输入该供应商的 API Key")
		}
		if _, err := c.newClient(profileClientConfig(profile, "")); err != nil {
			return fmt.Errorf("无法使用此供应商的 API 配置：%w", err)
		}
		if index < 0 {
			profile.ID = rand.Text()
			settings.Profiles = append(settings.Profiles, profile)
		} else {
			settings.Profiles[index] = profile
			for _, selection := range []*config.APISelection{&settings.Sidebar, &settings.Inline} {
				if selection.ProfileID == profile.ID && !slices.Contains(profile.Models, selection.Model) {
					selection.Model = ""
				}
			}
		}
		return nil
	})
	if err != nil {
		return APISettingsView{}, err
	}
	return c.refreshAPISettingsLocked(), nil
}

func customAPIBaseURL(input string) (string, error) {
	addr, err := url.Parse(strings.TrimSpace(input))
	if err != nil || (addr.Scheme != "http" && addr.Scheme != "https") || addr.Hostname() == "" ||
		addr.User != nil || addr.RawQuery != "" || addr.ForceQuery || addr.Fragment != "" {
		return "", errors.New("请输入完整的 HTTP 或 HTTPS API 地址，API Key 请单独填写")
	}
	addr.Host = strings.ToLower(addr.Host)
	base := strings.TrimRight(addr.String(), "/")
	return strings.TrimSuffix(base, "/chat/completions"), nil
}

func sameSupplier(a, b config.APIProfile) bool {
	return a.Provider == b.Provider && a.BaseURL == b.BaseURL
}

func (c *ChatService) SaveAPISelections(selections APISelections) (APISettingsView, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.config.UpdateAPISettings(func(settings *config.APISettings) error {
		for _, selection := range []config.APISelection{selections.Sidebar, selections.Inline} {
			if selection.ProfileID == "" && selection.Model == "" {
				continue
			}
			profile, ok := settings.Profile(selection.ProfileID)
			if !ok || profile.APIKey == "" {
				return errors.New("请先选择已设置 API Key 的供应商")
			}
			if selection.Model == "" {
				continue
			}
			if !slices.Contains(profile.Models, selection.Model) {
				return fmt.Errorf("模型已从「%s」移除，请重新选择", profile.Name)
			}
			if _, err := c.newClient(profileClientConfig(profile, selection.Model)); err != nil {
				return fmt.Errorf("无法使用 API 配置「%s」：%w", profile.Name, err)
			}
		}
		settings.Sidebar, settings.Inline = selections.Sidebar, selections.Inline
		return nil
	})
	if err != nil {
		return APISettingsView{}, err
	}
	return c.refreshAPISettingsLocked(), nil
}

func apiSettingsView(settings config.APISettings) APISettingsView {
	view := APISettingsView{
		Profiles:   make([]APIProfileView, 0, len(settings.Profiles)),
		Selections: APISelections{Sidebar: settings.Sidebar, Inline: settings.Inline},
	}
	for _, profile := range settings.Profiles {
		view.Profiles = append(view.Profiles, APIProfileView{
			ID: profile.ID, Name: profile.Name, Provider: profile.Provider, BaseURL: profile.BaseURL,
			Models: append([]string{}, profile.Models...), HasKey: profile.APIKey != "",
		})
	}
	return view
}

func (c *ChatService) refreshAPISettingsLocked() APISettingsView {
	settings := c.config.ReadAPISettings()
	c.applyAPISettingsLocked(settings)
	return apiSettingsView(settings)
}

func (c *ChatService) applyAPISettingsLocked(settings config.APISettings) {
	sidebar := c.resolveAPIRoute(settings, settings.Sidebar, c.sidebar)
	if sidebar.client != c.sidebar.client {
		c.conversation = nil
	}
	c.sidebar = sidebar
	c.inline = c.resolveAPIRoute(settings, settings.Inline, c.inline)
}

func (c *ChatService) resolveAPIRoute(settings config.APISettings, selection config.APISelection, current apiRoute) apiRoute {
	profile, ok := settings.Profile(selection.ProfileID)
	if !ok || selection.Model == "" || !slices.Contains(profile.Models, selection.Model) {
		return apiRoute{}
	}
	cfg := profileClientConfig(profile, selection.Model)
	if current.id == profile.ID && current.config == cfg {
		return current
	}
	client, err := c.newClient(cfg)
	if err != nil {
		return apiRoute{}
	}
	return apiRoute{id: profile.ID, config: cfg, client: client}
}

func profileClientConfig(profile config.APIProfile, model string) llmkit.Config {
	return llmkit.Config{
		Provider: llmkit.ProviderType(profile.Provider), APIKey: profile.APIKey, Model: model, BaseURL: profile.BaseURL,
	}
}
