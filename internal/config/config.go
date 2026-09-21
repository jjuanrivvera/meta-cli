package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultGraphVersion = "v26.0"

var (
	userConfigDir = os.UserConfigDir
	createTemp    = os.CreateTemp
)

type Account struct {
	BaseURL      string  `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	UploadURL    string  `yaml:"upload_url,omitempty" json:"upload_url,omitempty"`
	GraphVersion string  `yaml:"graph_version,omitempty" json:"graph_version,omitempty"`
	AuthMethod   string  `yaml:"auth_method,omitempty" json:"auth_method,omitempty"`
	BusinessID   string  `yaml:"business_id,omitempty" json:"business_id,omitempty"`
	PageID       string  `yaml:"page_id,omitempty" json:"page_id,omitempty"`
	InstagramID  string  `yaml:"instagram_id,omitempty" json:"instagram_id,omitempty"`
	WABAID       string  `yaml:"waba_id,omitempty" json:"waba_id,omitempty"`
	PhoneID      string  `yaml:"phone_id,omitempty" json:"phone_id,omitempty"`
	AppID        string  `yaml:"app_id,omitempty" json:"app_id,omitempty"`
	RequestsPS   float64 `yaml:"requests_per_second,omitempty" json:"requests_per_second,omitempty"`
}

type Config struct {
	Current  string             `yaml:"current,omitempty" json:"current,omitempty"`
	Accounts map[string]Account `yaml:"accounts,omitempty" json:"accounts,omitempty"`
	Aliases  map[string]string  `yaml:"aliases,omitempty" json:"aliases,omitempty"`
}

func Path() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(dir, "meta", "config.yaml"), nil
}

func Load(configPath string) (*Config, error) {
	if configPath == "" {
		var err error
		configPath, err = Path()
		if err != nil {
			return nil, err
		}
	}
	data, err := os.ReadFile(configPath) // #nosec G304 -- this is the user's selected config path
	if errors.Is(err, os.ErrNotExist) {
		return &Config{Accounts: map[string]Account{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var value Config
	if err := yaml.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if value.Accounts == nil {
		value.Accounts = map[string]Account{}
	}
	if value.Aliases == nil {
		value.Aliases = map[string]string{}
	}
	return &value, nil
}

func Save(configPath string, value *Config) error {
	if configPath == "" {
		var err error
		configPath, err = Path()
		if err != nil {
			return err
		}
	}
	for name, account := range value.Accounts {
		if err := ValidateAccount(name, account); err != nil {
			return err
		}
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	temporary, err := createTemp(dir, ".config-*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync config: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(temporaryPath, configPath); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func ValidateAccount(name string, account Account) error {
	if name == "" || strings.ContainsAny(name, `/\:*?"<>|`) || name == "." || name == ".." {
		return fmt.Errorf("invalid account name %q", name)
	}
	for label, raw := range map[string]string{"base URL": account.BaseURL, "upload URL": account.UploadURL} {
		if raw == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("%s must be an absolute HTTP or HTTPS URL", label)
		}
		if parsed.Scheme == "http" {
			host := parsed.Hostname()
			ip := net.ParseIP(host)
			if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
				return fmt.Errorf("%s may use plain HTTP only for loopback hosts", label)
			}
		}
	}
	return nil
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
