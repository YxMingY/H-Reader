package config

import (
	"encoding/json"
	"slices"
	"strings"
)

const apiSettingsVersion = 2

type APIProfile struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Provider string   `json:"provider"`
	BaseURL  string   `json:"base_url,omitempty"`
	Models   []string `json:"models"`
	APIKey   string   `json:"api_key"`
}

type APISelection struct {
	ProfileID string `json:"profileId"`
	Model     string `json:"model"`
}

type APISettings struct {
	Version  int          `json:"version"`
	Profiles []APIProfile `json:"profiles"`
	Sidebar  APISelection `json:"sidebar"`
	Inline   APISelection `json:"inline"`
}

type legacyAPIProfile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key"`
}

type legacyLLMConfig struct {
	APIKey   string            `json:"api_key"`
	APIKeys  map[string]string `json:"api_keys"`
	Provider string            `json:"provider"`
	Model    string            `json:"model"`
}

func (s APISettings) Profile(id string) (APIProfile, bool) {
	for _, profile := range s.Profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return APIProfile{}, false
}

func (c *Config) ReadAPISettings() APISettings {
	dataMutex.Lock()
	defer dataMutex.Unlock()
	return c.copyAPISettings()
}

// UpdateAPISettings writes the whole change before publishing it to readers.
func (c *Config) UpdateAPISettings(update func(*APISettings) error) error {
	dataMutex.Lock()
	defer dataMutex.Unlock()

	settings := c.copyAPISettings()
	if err := update(&settings); err != nil {
		return err
	}
	settings.Version = apiSettingsVersion
	next := *c
	next.API = &settings
	if err := next.saveToFileLocked(); err != nil {
		return err
	}
	c.API = next.API
	return nil
}

func (c *Config) copyAPISettings() APISettings {
	if c.API == nil {
		return APISettings{}
	}
	settings := *c.API
	settings.Profiles = slices.Clone(settings.Profiles)
	for i := range settings.Profiles {
		settings.Profiles[i].Models = slices.Clone(settings.Profiles[i].Models)
	}
	return settings
}

func (c *Config) decode(data []byte) (bool, error) {
	var legacy struct {
		*Config
		legacyLLMConfig
		API json.RawMessage `json:"api"`
	}
	var decoded Config
	legacy.Config = &decoded
	if err := json.Unmarshal(data, &legacy); err != nil {
		return false, err
	}
	*c = decoded
	if len(legacy.API) != 0 && string(legacy.API) != "null" {
		var settings APISettings
		if err := json.Unmarshal(legacy.API, &settings); err != nil {
			return false, err
		}
		if settings.Version >= apiSettingsVersion {
			c.API = &settings
			return false, nil
		}
		var old struct {
			Profiles  []legacyAPIProfile `json:"profiles"`
			SidebarID string             `json:"sidebar_profile_id"`
			InlineID  string             `json:"inline_profile_id"`
		}
		if err := json.Unmarshal(legacy.API, &old); err != nil {
			return false, err
		}
		c.API = migrateAPIProfiles(old.Profiles, old.SidebarID, old.InlineID)
		return true, nil
	}
	if legacy.APIKey == "" && len(legacy.APIKeys) == 0 && legacy.Provider == "" && legacy.Model == "" {
		return false, nil
	}
	profiles, activeID := legacyLLMProfiles(legacy.legacyLLMConfig)
	c.API = migrateAPIProfiles(profiles, activeID, activeID)
	return true, nil
}

func migrateAPIProfiles(profiles []legacyAPIProfile, sidebarID, inlineID string) *APISettings {
	settings := &APISettings{Version: apiSettingsVersion, Profiles: []APIProfile{}}
	for _, old := range profiles {
		provider := old.Provider
		if provider == "zhipu" {
			provider = "glm"
		}
		key := strings.TrimSpace(old.APIKey)
		index := slices.IndexFunc(settings.Profiles, func(profile APIProfile) bool {
			return profile.Provider == provider && profile.APIKey == key
		})
		if index < 0 {
			index = len(settings.Profiles)
			settings.Profiles = append(settings.Profiles, APIProfile{
				ID: old.ID, Name: old.Name, Provider: provider, APIKey: key, Models: []string{},
			})
		}
		profile := &settings.Profiles[index]
		model := strings.TrimSpace(old.Model)
		if model != "" && !slices.Contains(profile.Models, model) {
			profile.Models = append(profile.Models, model)
		}
		if old.ID == sidebarID {
			settings.Sidebar = APISelection{ProfileID: profile.ID, Model: model}
		}
		if old.ID == inlineID {
			settings.Inline = APISelection{ProfileID: profile.ID, Model: model}
		}
	}
	return settings
}

func legacyLLMProfiles(legacy legacyLLMConfig) ([]legacyAPIProfile, string) {
	activeProvider := legacy.Provider
	if activeProvider == "" {
		activeProvider = "aliyun"
	}
	if strings.TrimSpace(legacy.APIKey) != "" && strings.TrimSpace(legacy.APIKeys["aliyun"]) == "" {
		if legacy.APIKeys == nil {
			legacy.APIKeys = make(map[string]string)
		}
		legacy.APIKeys["aliyun"] = legacy.APIKey
	}

	var profiles []legacyAPIProfile
	var activeID string
	providers := make([]string, 0, len(legacy.APIKeys))
	for provider := range legacy.APIKeys {
		providers = append(providers, provider)
	}
	slices.Sort(providers)
	for _, provider := range providers {
		key := strings.TrimSpace(legacy.APIKeys[provider])
		if key == "" {
			continue
		}
		profile := legacyAPIProfile{
			ID: "legacy-" + provider, Name: provider + " 配置", Provider: provider,
			Model: legacyDefaultModel(provider), APIKey: key,
		}
		if provider == activeProvider {
			profile.Name = "原有配置"
			if legacy.Model != "" {
				profile.Model = legacy.Model
			}
			activeID = profile.ID
		}
		profiles = append(profiles, profile)
	}
	if activeID == "" && (legacy.Provider != "" || legacy.Model != "") {
		model := legacy.Model
		if model == "" {
			model = legacyDefaultModel(activeProvider)
		}
		profiles = append(profiles, legacyAPIProfile{
			ID: "legacy-" + activeProvider, Name: "原有配置", Provider: activeProvider, Model: model,
		})
	}
	return profiles, activeID
}

func legacyDefaultModel(provider string) string {
	switch provider {
	case "aliyun":
		return "qwen3.8-flash"
	case "deepseek":
		return "deepseek-flash"
	case "glm", "zhipu":
		return "glm-5.3-flash"
	default:
		return ""
	}
}
