package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultProfileName is the profile used when no --profile flag or CUBBIT_PROFILE is given.
const DefaultProfileName = "default"

// LastDeploy holds metadata about the most recent successful deploy.
type LastDeploy struct {
	Bucket    string    `yaml:"bucket"`
	Prefix    string    `yaml:"prefix,omitempty"`
	URL       string    `yaml:"url"`
	Files     int       `yaml:"files"`
	Encrypted bool      `yaml:"encrypted"`
	Date      time.Time `yaml:"date"`
}

// ProfileConfig holds credentials and settings for a single named profile.
type ProfileConfig struct {
	AccessKey  string      `yaml:"access_key"`
	SecretKey  string      `yaml:"secret_key"`
	Bucket     string      `yaml:"bucket"`
	Endpoint   string      `yaml:"endpoint,omitempty"`
	Locale     string      `yaml:"locale,omitempty"`
	LastDeploy *LastDeploy `yaml:"last_deploy,omitempty"`
}

// FileConfig is the on-disk representation of ~/.cubbit/pages/config.yaml.
// It holds named profiles. If Default is set it overrides DefaultProfileName as the
// active profile when no --profile flag or CUBBIT_PROFILE env var is given.
type FileConfig struct {
	Default  string                    `yaml:"default,omitempty"`
	Profiles map[string]*ProfileConfig `yaml:"profiles,omitempty"`
}

// GetProfile returns the profile with the given name, or nil if not found.
func (fc *FileConfig) GetProfile(name string) *ProfileConfig {
	if fc == nil || fc.Profiles == nil {
		return nil
	}
	return fc.Profiles[name]
}

// SetProfile creates or replaces a profile entry.
func (fc *FileConfig) SetProfile(name string, pc *ProfileConfig) {
	if fc.Profiles == nil {
		fc.Profiles = make(map[string]*ProfileConfig)
	}
	fc.Profiles[name] = pc
}

// ActiveProfileName returns the effective profile name given an explicit override.
// Priority: explicit → CUBBIT_PROFILE env var → fc.Default → DefaultProfileName.
// Safe to call on a nil receiver.
func (fc *FileConfig) ActiveProfileName(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if v := os.Getenv("CUBBIT_PROFILE"); v != "" {
		return v
	}
	if fc != nil && fc.Default != "" {
		return fc.Default
	}
	return DefaultProfileName
}

// ProfileNames returns the sorted list of profile names.
// "default" appears first when present, followed by remaining names in alphabetical order.
// Safe to call on a nil receiver.
func (fc *FileConfig) ProfileNames() []string {
	if fc == nil || len(fc.Profiles) == 0 {
		return nil
	}
	names := make([]string, 0, len(fc.Profiles))
	for k := range fc.Profiles {
		names = append(names, k)
	}
	sort.SliceStable(names, func(i, j int) bool {
		if names[i] == DefaultProfileName && names[j] == DefaultProfileName {
			return false
		}
		if names[i] == DefaultProfileName {
			return true
		}
		if names[j] == DefaultProfileName {
			return false
		}
		return names[i] < names[j]
	})
	return names
}

// ConfigDir returns ~/.cubbit/pages/.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".cubbit", "pages"), nil
}

// ConfigFilePath returns ~/.cubbit/pages/config.yaml.
func ConfigFilePath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// legacyFileConfig detects the pre-profiles flat format.
type legacyFileConfig struct {
	AccessKey  string      `yaml:"access_key"`
	SecretKey  string      `yaml:"secret_key"`
	Bucket     string      `yaml:"bucket"`
	Endpoint   string      `yaml:"endpoint"`
	Locale     string      `yaml:"locale"`
	LastDeploy *LastDeploy `yaml:"last_deploy"`
}

// LoadFileConfig loads ~/.cubbit/pages/config.yaml.
// Automatically migrates the pre-profiles (flat) format to the current format.
// Returns nil (no error) if the file does not exist.
func LoadFileConfig() (*FileConfig, error) {
	path, err := ConfigFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Try new multi-profile format first.
	var fc FileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	if len(fc.Profiles) > 0 {
		return &fc, nil
	}

	// Try legacy flat format (pre-profiles).
	var legacy legacyFileConfig
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	if legacy.AccessKey != "" {
		fmt.Fprintf(os.Stderr, "info: migrating legacy config to multi-profile format (profile: %q)\n", DefaultProfileName)
		pc := &ProfileConfig{
			AccessKey:  legacy.AccessKey,
			SecretKey:  legacy.SecretKey,
			Bucket:     legacy.Bucket,
			Endpoint:   legacy.Endpoint,
			Locale:     legacy.Locale,
			LastDeploy: legacy.LastDeploy,
		}
		fc = FileConfig{}
		fc.SetProfile(DefaultProfileName, pc)
		return &fc, nil
	}

	return &fc, nil
}

// SaveFileConfig writes to ~/.cubbit/pages/config.yaml (mode 0600).
func SaveFileConfig(fc *FileConfig) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(fc)
	if err != nil {
		return fmt.Errorf("serializing config: %w", err)
	}

	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}
