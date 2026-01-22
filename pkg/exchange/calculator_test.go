package exchange

import (
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// Types Tests
// =============================================================================

func TestHardwareProfile_Key(t *testing.T) {
	tests := []struct {
		profile  HardwareProfile
		expected string
	}{
		{
			profile:  HardwareProfile{Vendor: VendorNVIDIA, Model: "A100-80GB"},
			expected: "nvidia/A100-80GB",
		},
		{
			profile:  HardwareProfile{Vendor: VendorHuawei, Model: "910B"},
			expected: "huawei/910B",
		},
		{
			profile:  HardwareProfile{Vendor: "custom", Model: "test-model"},
			expected: "custom/test-model",
		},
	}

	for _, tt := range tests {
		result := tt.profile.Key()
		if result != tt.expected {
			t.Errorf("Key() = %q, want %q", result, tt.expected)
		}
	}
}

func TestHardwareProfile_VRAMGiB(t *testing.T) {
	tests := []struct {
		vramBytes uint64
		expected  float64
	}{
		{80 * 1024 * 1024 * 1024, 80.0},
		{40 * 1024 * 1024 * 1024, 40.0},
		{24 * 1024 * 1024 * 1024, 24.0},
		{0, 0.0},
	}

	for _, tt := range tests {
		profile := HardwareProfile{VRAMBytes: tt.vramBytes}
		result := profile.VRAMGiB()
		if result != tt.expected {
			t.Errorf("VRAMGiB() with %d bytes = %f, want %f", tt.vramBytes, result, tt.expected)
		}
	}
}

func TestHardwareProfile_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		profile HardwareProfile
		valid   bool
	}{
		{
			name: "valid profile",
			profile: HardwareProfile{
				Vendor:    VendorNVIDIA,
				Model:     "A100-80GB",
				VRAMBytes: 80 * 1024 * 1024 * 1024,
			},
			valid: true,
		},
		{
			name: "missing vendor",
			profile: HardwareProfile{
				Model:     "A100-80GB",
				VRAMBytes: 80 * 1024 * 1024 * 1024,
			},
			valid: false,
		},
		{
			name: "missing model",
			profile: HardwareProfile{
				Vendor:    VendorNVIDIA,
				VRAMBytes: 80 * 1024 * 1024 * 1024,
			},
			valid: false,
		},
		{
			name: "zero vram",
			profile: HardwareProfile{
				Vendor: VendorNVIDIA,
				Model:  "A100-80GB",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.profile.IsValid()
			if result != tt.valid {
				t.Errorf("IsValid() = %v, want %v", result, tt.valid)
			}
		})
	}
}

func TestExchangeRate_Key(t *testing.T) {
	rate := ExchangeRate{
		BaseModel:   "A100-80GB",
		TargetModel: "RTX4090",
	}

	expected := "A100-80GB->RTX4090"
	if result := rate.Key(); result != expected {
		t.Errorf("Key() = %q, want %q", result, expected)
	}
}

func TestExchangeRate_Inverse(t *testing.T) {
	rate := ExchangeRate{
		BaseModel:    "A100-80GB",
		TargetModel:  "RTX4090",
		ComputeRatio: 0.5,
		MemoryRatio:  0.3,
	}

	inverse := rate.Inverse()

	if inverse.BaseModel != rate.TargetModel {
		t.Errorf("Inverse BaseModel = %q, want %q", inverse.BaseModel, rate.TargetModel)
	}
	if inverse.TargetModel != rate.BaseModel {
		t.Errorf("Inverse TargetModel = %q, want %q", inverse.TargetModel, rate.BaseModel)
	}
	if inverse.ComputeRatio != 2.0 {
		t.Errorf("Inverse ComputeRatio = %f, want 2.0", inverse.ComputeRatio)
	}
	if inverse.MemoryRatio < 3.33 || inverse.MemoryRatio > 3.34 {
		t.Errorf("Inverse MemoryRatio = %f, want ~3.33", inverse.MemoryRatio)
	}
}

func TestBuiltinProfiles(t *testing.T) {
	if len(BuiltinProfiles) == 0 {
		t.Fatal("BuiltinProfiles should not be empty")
	}

	// Verify A100-80GB exists (default base model)
	found := false
	for _, p := range BuiltinProfiles {
		if p.Model == DefaultBaseModel {
			found = true
			if p.Vendor != VendorNVIDIA {
				t.Errorf("A100-80GB should be NVIDIA, got %s", p.Vendor)
			}
			if p.FP16TFLOPS != 312 {
				t.Errorf("A100-80GB FP16TFLOPS = %f, want 312", p.FP16TFLOPS)
			}
			break
		}
	}
	if !found {
		t.Errorf("Default base model %q not found in BuiltinProfiles", DefaultBaseModel)
	}

	// Verify all builtin profiles are valid
	for i, p := range BuiltinProfiles {
		if !p.IsValid() {
			t.Errorf("BuiltinProfiles[%d] (%s) is invalid", i, p.Key())
		}
	}
}

// =============================================================================
// Calculator Tests
// =============================================================================

func TestNewCalculator(t *testing.T) {
	calc := NewCalculator()

	if calc == nil {
		t.Fatal("NewCalculator() returned nil")
	}

	if calc.GetBaseModel() != DefaultBaseModel {
		t.Errorf("Default base model = %q, want %q", calc.GetBaseModel(), DefaultBaseModel)
	}

	profiles := calc.ListProfiles()
	if len(profiles) != len(BuiltinProfiles) {
		t.Errorf("Expected %d profiles, got %d", len(BuiltinProfiles), len(profiles))
	}
}

func TestNewCalculatorWithConfig(t *testing.T) {
	config := &ProfileConfig{
		BaseModel: "RTX4090",
		Profiles: []HardwareProfile{
			{
				Vendor:     VendorNVIDIA,
				Model:      "RTX4090",
				FP16TFLOPS: 165,
				VRAMBytes:  24 * 1024 * 1024 * 1024,
			},
		},
	}

	calc := NewCalculatorWithConfig(config)

	if calc.GetBaseModel() != "RTX4090" {
		t.Errorf("Base model = %q, want RTX4090", calc.GetBaseModel())
	}

	profiles := calc.ListProfiles()
	if len(profiles) != 1 {
		t.Errorf("Expected 1 profile, got %d", len(profiles))
	}
}

func TestNewCalculatorWithConfig_Nil(t *testing.T) {
	calc := NewCalculatorWithConfig(nil)

	// Should fall back to builtin profiles
	profiles := calc.ListProfiles()
	if len(profiles) != len(BuiltinProfiles) {
		t.Errorf("Expected %d profiles with nil config, got %d", len(BuiltinProfiles), len(profiles))
	}
}

func TestNewCalculatorWithConfig_EmptyProfiles(t *testing.T) {
	config := &ProfileConfig{
		Profiles: []HardwareProfile{},
	}

	calc := NewCalculatorWithConfig(config)

	// Should fall back to builtin profiles
	profiles := calc.ListProfiles()
	if len(profiles) != len(BuiltinProfiles) {
		t.Errorf("Expected %d profiles with empty config, got %d", len(BuiltinProfiles), len(profiles))
	}
}

func TestCalculator_GetProfile(t *testing.T) {
	calc := NewCalculator()

	// Existing profile
	profile := calc.GetProfile(VendorNVIDIA, "A100-80GB")
	if profile == nil {
		t.Fatal("GetProfile(nvidia, A100-80GB) returned nil")
	}
	if profile.Model != "A100-80GB" {
		t.Errorf("Profile model = %q, want A100-80GB", profile.Model)
	}

	// Non-existing profile
	profile = calc.GetProfile("unknown", "unknown")
	if profile != nil {
		t.Error("GetProfile for unknown should return nil")
	}
}

func TestCalculator_GetProfileByModel(t *testing.T) {
	calc := NewCalculator()

	// Existing model
	profile := calc.GetProfileByModel("A100-80GB")
	if profile == nil {
		t.Fatal("GetProfileByModel(A100-80GB) returned nil")
	}
	if profile.Vendor != VendorNVIDIA {
		t.Errorf("Profile vendor = %q, want nvidia", profile.Vendor)
	}

	// Non-existing model
	profile = calc.GetProfileByModel("unknown")
	if profile != nil {
		t.Error("GetProfileByModel for unknown should return nil")
	}
}

func TestCalculator_GetRate_SameModel(t *testing.T) {
	calc := NewCalculator()

	rate, err := calc.GetRate("A100-80GB", "A100-80GB")
	if err != nil {
		t.Fatalf("GetRate same model error: %v", err)
	}

	if rate.ComputeRatio != 1.0 {
		t.Errorf("Same model ComputeRatio = %f, want 1.0", rate.ComputeRatio)
	}
	if rate.MemoryRatio != 1.0 {
		t.Errorf("Same model MemoryRatio = %f, want 1.0", rate.MemoryRatio)
	}
}

func TestCalculator_GetRate_FromBaseModel(t *testing.T) {
	calc := NewCalculator()

	rate, err := calc.GetRate("A100-80GB", "RTX4090")
	if err != nil {
		t.Fatalf("GetRate from base model error: %v", err)
	}

	// RTX4090 has 165 FP16 TFLOPS vs A100's 312
	expectedRatio := 165.0 / 312.0
	if rate.ComputeRatio < expectedRatio-0.01 || rate.ComputeRatio > expectedRatio+0.01 {
		t.Errorf("ComputeRatio = %f, want ~%f", rate.ComputeRatio, expectedRatio)
	}
}

func TestCalculator_GetRate_ToBaseModel(t *testing.T) {
	calc := NewCalculator()

	rate, err := calc.GetRate("RTX4090", "A100-80GB")
	if err != nil {
		t.Fatalf("GetRate to base model error: %v", err)
	}

	// Inverse of base->target
	expectedRatio := 312.0 / 165.0
	if rate.ComputeRatio < expectedRatio-0.01 || rate.ComputeRatio > expectedRatio+0.01 {
		t.Errorf("ComputeRatio = %f, want ~%f", rate.ComputeRatio, expectedRatio)
	}
}

func TestCalculator_GetRate_CrossModels(t *testing.T) {
	calc := NewCalculator()

	// RTX4090 -> 910B (both non-base)
	rate, err := calc.GetRate("RTX4090", "910B")
	if err != nil {
		t.Fatalf("GetRate cross models error: %v", err)
	}

	// Should calculate through base model
	if rate.ComputeRatio <= 0 {
		t.Error("Cross model ComputeRatio should be positive")
	}
}

func TestCalculator_GetRate_UnknownModel(t *testing.T) {
	calc := NewCalculator()

	_, err := calc.GetRate("A100-80GB", "unknown")
	if err == nil {
		t.Error("GetRate with unknown model should return error")
	}

	_, err = calc.GetRate("unknown", "A100-80GB")
	if err == nil {
		t.Error("GetRate from unknown model should return error")
	}
}

func TestCalculator_NormalizeCompute(t *testing.T) {
	calc := NewCalculator()

	// Test with A100 (base model)
	normalized, err := calc.NormalizeCompute(VendorNVIDIA, "A100-80GB", 2)
	if err != nil {
		t.Fatalf("NormalizeCompute error: %v", err)
	}

	if normalized.BaseModel != DefaultBaseModel {
		t.Errorf("BaseModel = %q, want %q", normalized.BaseModel, DefaultBaseModel)
	}
	if normalized.OriginalModel != "A100-80GB" {
		t.Errorf("OriginalModel = %q, want A100-80GB", normalized.OriginalModel)
	}
	// 2 A100s = 2 base model units
	if normalized.NormalizedTFLOPS != 2.0 {
		t.Errorf("NormalizedTFLOPS = %f, want 2.0", normalized.NormalizedTFLOPS)
	}
}

func TestCalculator_NormalizeCompute_ByModelOnly(t *testing.T) {
	calc := NewCalculator()

	// Should find by model name when vendor doesn't match
	normalized, err := calc.NormalizeCompute("wrong-vendor", "A100-80GB", 1)
	if err != nil {
		t.Fatalf("NormalizeCompute by model error: %v", err)
	}

	if normalized.NormalizedTFLOPS != 1.0 {
		t.Errorf("NormalizedTFLOPS = %f, want 1.0", normalized.NormalizedTFLOPS)
	}
}

func TestCalculator_NormalizeCompute_Unknown(t *testing.T) {
	calc := NewCalculator()

	_, err := calc.NormalizeCompute("unknown", "unknown", 1)
	if err == nil {
		t.Error("NormalizeCompute with unknown hardware should return error")
	}
}

func TestCalculator_ConvertCompute(t *testing.T) {
	calc := NewCalculator()

	// Convert 1.0 base TFLOPS to RTX4090 equivalent
	result, err := calc.ConvertCompute(1.0, "RTX4090")
	if err != nil {
		t.Fatalf("ConvertCompute error: %v", err)
	}

	// RTX4090 has lower compute ratio, so needs more TFLOPS
	if result <= 1.0 {
		t.Errorf("ConvertCompute to RTX4090 should require more than 1.0, got %f", result)
	}
}

func TestCalculator_ConvertVRAM(t *testing.T) {
	calc := NewCalculator()

	vram := uint64(80 * 1024 * 1024 * 1024)
	result, err := calc.ConvertVRAM(vram, "RTX4090")
	if err != nil {
		t.Fatalf("ConvertVRAM error: %v", err)
	}

	// Current implementation returns same bytes
	if result != vram {
		t.Errorf("ConvertVRAM = %d, want %d", result, vram)
	}
}

func TestCalculator_AddProfile(t *testing.T) {
	calc := NewCalculator()

	newProfile := &HardwareProfile{
		Vendor:     "test",
		Model:      "test-gpu",
		FP16TFLOPS: 100,
		VRAMBytes:  16 * 1024 * 1024 * 1024,
	}

	err := calc.AddProfile(newProfile)
	if err != nil {
		t.Fatalf("AddProfile error: %v", err)
	}

	// Verify it was added
	profile := calc.GetProfile("test", "test-gpu")
	if profile == nil {
		t.Error("Added profile not found")
	}
}

func TestCalculator_AddProfile_Invalid(t *testing.T) {
	calc := NewCalculator()

	invalidProfile := &HardwareProfile{
		Model: "test-gpu", // Missing vendor
	}

	err := calc.AddProfile(invalidProfile)
	if err == nil {
		t.Error("AddProfile with invalid profile should return error")
	}
}

func TestCalculator_SetBaseModel(t *testing.T) {
	calc := NewCalculator()

	err := calc.SetBaseModel("H100-80GB")
	if err != nil {
		t.Fatalf("SetBaseModel error: %v", err)
	}

	if calc.GetBaseModel() != "H100-80GB" {
		t.Errorf("Base model = %q, want H100-80GB", calc.GetBaseModel())
	}
}

func TestCalculator_SetBaseModel_Unknown(t *testing.T) {
	calc := NewCalculator()

	err := calc.SetBaseModel("unknown")
	if err == nil {
		t.Error("SetBaseModel with unknown model should return error")
	}
}

func TestCalculator_ListProfiles(t *testing.T) {
	calc := NewCalculator()

	profiles := calc.ListProfiles()
	if len(profiles) == 0 {
		t.Error("ListProfiles should not return empty list")
	}

	// Verify all profiles have valid keys
	for _, p := range profiles {
		if p.Key() == "/" {
			t.Error("Profile has empty key")
		}
	}
}

func TestCalculator_ListRates(t *testing.T) {
	calc := NewCalculator()

	rates := calc.ListRates()
	// Should have rates for all non-base profiles
	expectedRates := len(BuiltinProfiles) - 1
	if len(rates) != expectedRates {
		t.Errorf("ListRates returned %d rates, want %d", len(rates), expectedRates)
	}
}

func TestCalculator_ScoreNode(t *testing.T) {
	calc := NewCalculator()

	// Score A100 node with 2 devices
	score := calc.ScoreNode(VendorNVIDIA, "A100-80GB", 2, 160*1024*1024*1024)
	if score <= 0 {
		t.Error("ScoreNode should return positive score")
	}

	// Higher device count should give higher score
	score2 := calc.ScoreNode(VendorNVIDIA, "A100-80GB", 4, 320*1024*1024*1024)
	if score2 <= score {
		t.Error("More devices should give higher score")
	}
}

func TestCalculator_ScoreNode_UnknownHardware(t *testing.T) {
	calc := NewCalculator()

	score := calc.ScoreNode("unknown", "unknown", 1, 0)
	if score != 0 {
		t.Errorf("ScoreNode for unknown hardware should return 0, got %f", score)
	}
}

// =============================================================================
// Config Tests
// =============================================================================

func TestLoadProfilesFromData(t *testing.T) {
	yamlData := []byte(`
profiles:
  - vendor: nvidia
    model: TestGPU
    fp16_tflops: 200
    fp32_tflops: 100
    vram_bytes: 34359738368
    mem_bw_gbps: 1000
    tdp_watts: 300
base_model: TestGPU
`)

	config, err := LoadProfilesFromData(yamlData)
	if err != nil {
		t.Fatalf("LoadProfilesFromData error: %v", err)
	}

	if len(config.Profiles) != 1 {
		t.Errorf("Expected 1 profile, got %d", len(config.Profiles))
	}

	if config.BaseModel != "TestGPU" {
		t.Errorf("BaseModel = %q, want TestGPU", config.BaseModel)
	}

	profile := config.Profiles[0]
	if profile.Vendor != "nvidia" {
		t.Errorf("Vendor = %q, want nvidia", profile.Vendor)
	}
	if profile.FP16TFLOPS != 200 {
		t.Errorf("FP16TFLOPS = %f, want 200", profile.FP16TFLOPS)
	}
}

func TestLoadProfilesFromData_Empty(t *testing.T) {
	_, err := LoadProfilesFromData([]byte{})
	if err == nil {
		t.Error("LoadProfilesFromData with empty data should return error")
	}
}

func TestLoadProfilesFromData_InvalidYAML(t *testing.T) {
	_, err := LoadProfilesFromData([]byte("not: valid: yaml: :::"))
	if err == nil {
		t.Error("LoadProfilesFromData with invalid YAML should return error")
	}
}

func TestLoadProfilesFromData_InvalidProfile(t *testing.T) {
	yamlData := []byte(`
profiles:
  - vendor: nvidia
    model: ""
    vram_bytes: 0
`)

	_, err := LoadProfilesFromData(yamlData)
	if err == nil {
		t.Error("LoadProfilesFromData with invalid profile should return error")
	}
}

func TestValidateProfileConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *ProfileConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "valid config",
			config: &ProfileConfig{
				Profiles: []HardwareProfile{
					{Vendor: "nvidia", Model: "A100", VRAMBytes: 80 * 1024 * 1024 * 1024, FP16TFLOPS: 312},
				},
				BaseModel: "A100",
			},
			wantErr: false,
		},
		{
			name: "base model not in profiles",
			config: &ProfileConfig{
				Profiles: []HardwareProfile{
					{Vendor: "nvidia", Model: "A100", VRAMBytes: 80 * 1024 * 1024 * 1024, FP16TFLOPS: 312},
				},
				BaseModel: "RTX4090",
			},
			wantErr: true,
		},
		{
			name: "empty base model is ok",
			config: &ProfileConfig{
				Profiles: []HardwareProfile{
					{Vendor: "nvidia", Model: "A100", VRAMBytes: 80 * 1024 * 1024 * 1024, FP16TFLOPS: 312},
				},
			},
			wantErr: false,
		},
		{
			name: "empty profiles with base model",
			config: &ProfileConfig{
				Profiles:  []HardwareProfile{},
				BaseModel: "A100",
			},
			wantErr: false, // Empty profiles + base model is allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProfileConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProfileConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile *HardwareProfile
		wantErr bool
	}{
		{
			name: "valid profile",
			profile: &HardwareProfile{
				Vendor:     "nvidia",
				Model:      "A100",
				VRAMBytes:  80 * 1024 * 1024 * 1024,
				FP16TFLOPS: 312,
			},
			wantErr: false,
		},
		{
			name: "missing vendor",
			profile: &HardwareProfile{
				Model:      "A100",
				VRAMBytes:  80 * 1024 * 1024 * 1024,
				FP16TFLOPS: 312,
			},
			wantErr: true,
		},
		{
			name: "missing model",
			profile: &HardwareProfile{
				Vendor:     "nvidia",
				VRAMBytes:  80 * 1024 * 1024 * 1024,
				FP16TFLOPS: 312,
			},
			wantErr: true,
		},
		{
			name: "zero vram",
			profile: &HardwareProfile{
				Vendor:     "nvidia",
				Model:      "A100",
				FP16TFLOPS: 312,
			},
			wantErr: true,
		},
		{
			name: "no compute metrics",
			profile: &HardwareProfile{
				Vendor:    "nvidia",
				Model:     "A100",
				VRAMBytes: 80 * 1024 * 1024 * 1024,
			},
			wantErr: true,
		},
		{
			name: "only FP32 is valid",
			profile: &HardwareProfile{
				Vendor:     "nvidia",
				Model:      "A100",
				VRAMBytes:  80 * 1024 * 1024 * 1024,
				FP32TFLOPS: 19.5,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProfile(tt.profile)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProfile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMergeProfiles(t *testing.T) {
	// Merge with nil custom config
	merged := MergeProfiles(nil)
	if merged.BaseModel != DefaultBaseModel {
		t.Errorf("Merged base model = %q, want %q", merged.BaseModel, DefaultBaseModel)
	}
	if len(merged.Profiles) != len(BuiltinProfiles) {
		t.Errorf("Merged profiles count = %d, want %d", len(merged.Profiles), len(BuiltinProfiles))
	}
}

func TestMergeProfiles_Override(t *testing.T) {
	custom := &ProfileConfig{
		BaseModel: "custom-base",
		Profiles: []HardwareProfile{
			{
				Vendor:     VendorNVIDIA,
				Model:      "A100-80GB", // Override existing
				FP16TFLOPS: 999,
				VRAMBytes:  80 * 1024 * 1024 * 1024,
			},
		},
	}

	merged := MergeProfiles(custom)

	if merged.BaseModel != "custom-base" {
		t.Errorf("Merged base model = %q, want custom-base", merged.BaseModel)
	}

	// Find the A100-80GB profile
	var found *HardwareProfile
	for i := range merged.Profiles {
		if merged.Profiles[i].Model == "A100-80GB" {
			found = &merged.Profiles[i]
			break
		}
	}

	if found == nil {
		t.Fatal("A100-80GB not found in merged profiles")
	}

	if found.FP16TFLOPS != 999 {
		t.Errorf("A100-80GB FP16TFLOPS = %f, want 999 (overridden)", found.FP16TFLOPS)
	}
}

func TestMergeProfiles_InvalidCustom(t *testing.T) {
	custom := &ProfileConfig{
		Profiles: []HardwareProfile{
			{
				Vendor: VendorNVIDIA,
				// Missing Model and VRAMBytes - invalid
			},
		},
	}

	merged := MergeProfiles(custom)

	// Invalid profiles should be skipped
	for _, p := range merged.Profiles {
		if p.Model == "" {
			t.Error("Invalid profile should not be included")
		}
	}
}

func TestNewCalculatorFromData(t *testing.T) {
	yamlData := []byte(`
profiles:
  - vendor: nvidia
    model: TestGPU
    fp16_tflops: 200
    vram_bytes: 34359738368
base_model: TestGPU
`)

	calc, err := NewCalculatorFromData(yamlData)
	if err != nil {
		t.Fatalf("NewCalculatorFromData error: %v", err)
	}

	// Should have merged with builtin profiles
	profiles := calc.ListProfiles()
	if len(profiles) <= 1 {
		t.Error("Should have merged with builtin profiles")
	}

	// Custom profile should exist
	profile := calc.GetProfile("nvidia", "TestGPU")
	if profile == nil {
		t.Error("Custom profile not found")
	}
}

func TestLoadProfilesFromFile(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "profiles.yaml")

	yamlData := `
profiles:
  - vendor: nvidia
    model: TestGPU
    fp16_tflops: 200
    vram_bytes: 34359738368
base_model: TestGPU
`
	if err := os.WriteFile(tmpFile, []byte(yamlData), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	config, err := LoadProfilesFromFile(tmpFile)
	if err != nil {
		t.Fatalf("LoadProfilesFromFile error: %v", err)
	}

	if len(config.Profiles) != 1 {
		t.Errorf("Expected 1 profile, got %d", len(config.Profiles))
	}
}

func TestLoadProfilesFromFile_NotFound(t *testing.T) {
	_, err := LoadProfilesFromFile("/nonexistent/path/profiles.yaml")
	if err == nil {
		t.Error("LoadProfilesFromFile with nonexistent file should return error")
	}
}

func TestNewCalculatorFromFile(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "profiles.yaml")

	yamlData := `
profiles:
  - vendor: nvidia
    model: TestGPU
    fp16_tflops: 200
    vram_bytes: 34359738368
`
	if err := os.WriteFile(tmpFile, []byte(yamlData), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	calc, err := NewCalculatorFromFile(tmpFile)
	if err != nil {
		t.Fatalf("NewCalculatorFromFile error: %v", err)
	}

	// Should have merged with builtin profiles
	profile := calc.GetProfile("nvidia", "TestGPU")
	if profile == nil {
		t.Error("Custom profile not found")
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestCalculator_Concurrency(t *testing.T) {
	calc := NewCalculator()
	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				calc.GetProfile(VendorNVIDIA, "A100-80GB")
				calc.GetProfileByModel("RTX4090")
				calc.GetBaseModel()
				calc.ListProfiles()
				calc.ListRates()
			}
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 20; j++ {
				profile := &HardwareProfile{
					Vendor:     "test",
					Model:      "concurrent-test",
					FP16TFLOPS: 100,
					VRAMBytes:  16 * 1024 * 1024 * 1024,
				}
				calc.AddProfile(profile)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}
}
