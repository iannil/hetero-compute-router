package exchange

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

// LoadProfilesFromFile loads hardware profiles from a YAML configuration file.
func LoadProfilesFromFile(path string) (*ProfileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	return LoadProfilesFromData(data)
}

// LoadProfilesFromData parses hardware profiles from YAML data.
// This is useful for loading from Kubernetes ConfigMap data.
func LoadProfilesFromData(data []byte) (*ProfileConfig, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty configuration data")
	}

	var config ProfileConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := ValidateProfileConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// ValidateProfileConfig validates a ProfileConfig for correctness.
func ValidateProfileConfig(config *ProfileConfig) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	// Validate each profile
	for i, profile := range config.Profiles {
		if err := ValidateProfile(&profile); err != nil {
			return fmt.Errorf("profile[%d]: %w", i, err)
		}
	}

	// If base model is specified, verify it exists in profiles
	if config.BaseModel != "" {
		found := false
		for _, profile := range config.Profiles {
			if profile.Model == config.BaseModel {
				found = true
				break
			}
		}
		if !found && len(config.Profiles) > 0 {
			return fmt.Errorf("base model %q not found in profiles", config.BaseModel)
		}
	}

	return nil
}

// ValidateProfile validates a single HardwareProfile.
func ValidateProfile(profile *HardwareProfile) error {
	if profile.Vendor == "" {
		return fmt.Errorf("vendor is required")
	}

	if profile.Model == "" {
		return fmt.Errorf("model is required")
	}

	if profile.VRAMBytes == 0 {
		return fmt.Errorf("vram_bytes must be positive")
	}

	// At least one compute metric should be specified
	if profile.FP16TFLOPS == 0 && profile.FP32TFLOPS == 0 {
		return fmt.Errorf("at least one of fp16_tflops or fp32_tflops must be specified")
	}

	return nil
}

// MergeProfiles merges custom profiles with builtin profiles.
// Custom profiles take precedence over builtin ones with the same key.
func MergeProfiles(custom *ProfileConfig) *ProfileConfig {
	merged := &ProfileConfig{
		BaseModel: DefaultBaseModel,
		Profiles:  make([]HardwareProfile, 0),
	}

	// Start with builtin profiles
	profileMap := make(map[string]HardwareProfile)
	for _, p := range BuiltinProfiles {
		profileMap[p.Key()] = p
	}

	// Override with custom profiles
	if custom != nil {
		if custom.BaseModel != "" {
			merged.BaseModel = custom.BaseModel
		}

		for _, p := range custom.Profiles {
			if p.IsValid() {
				profileMap[p.Key()] = p
			}
		}
	}

	// Convert map back to slice
	for _, p := range profileMap {
		merged.Profiles = append(merged.Profiles, p)
	}

	return merged
}

// LoadAndMergeProfiles loads profiles from a file and merges with builtins.
func LoadAndMergeProfiles(path string) (*ProfileConfig, error) {
	custom, err := LoadProfilesFromFile(path)
	if err != nil {
		return nil, err
	}

	return MergeProfiles(custom), nil
}

// NewCalculatorFromFile creates a Calculator from a YAML configuration file.
// It merges custom profiles with builtin profiles.
func NewCalculatorFromFile(path string) (*Calculator, error) {
	config, err := LoadAndMergeProfiles(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load profiles: %w", err)
	}

	return NewCalculatorWithConfig(config), nil
}

// NewCalculatorFromData creates a Calculator from YAML configuration data.
// It merges custom profiles with builtin profiles.
func NewCalculatorFromData(data []byte) (*Calculator, error) {
	custom, err := LoadProfilesFromData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse profiles: %w", err)
	}

	merged := MergeProfiles(custom)
	return NewCalculatorWithConfig(merged), nil
}
