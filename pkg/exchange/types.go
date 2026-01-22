package exchange

// Vendor constants for supported hardware vendors
const (
	VendorNVIDIA    = "nvidia"
	VendorHuawei    = "huawei"
	VendorHygon     = "hygon"
	VendorCambricon = "cambricon"
)

// HardwareProfile defines the compute parameters for a single hardware model.
// This is the core data structure for compute power normalization across vendors.
type HardwareProfile struct {
	// Vendor is the hardware vendor identifier (nvidia, huawei, hygon, cambricon)
	Vendor string `json:"vendor" yaml:"vendor"`

	// Model is the specific hardware model name (e.g., A100-80GB, 910B)
	Model string `json:"model" yaml:"model"`

	// FP16TFLOPS is the theoretical peak FP16 performance in TFLOPS
	FP16TFLOPS float64 `json:"fp16_tflops" yaml:"fp16_tflops"`

	// FP32TFLOPS is the theoretical peak FP32 performance in TFLOPS
	FP32TFLOPS float64 `json:"fp32_tflops" yaml:"fp32_tflops"`

	// VRAMBytes is the total video memory in bytes
	VRAMBytes uint64 `json:"vram_bytes" yaml:"vram_bytes"`

	// MemBWGBps is the memory bandwidth in GB/s
	MemBWGBps float64 `json:"mem_bw_gbps" yaml:"mem_bw_gbps"`

	// TDPWatts is the thermal design power in watts
	TDPWatts int `json:"tdp_watts" yaml:"tdp_watts"`
}

// ExchangeRate defines the conversion ratio between two hardware models.
// It enables "compute currency" conversion for cross-vendor scheduling.
type ExchangeRate struct {
	// BaseModel is the reference model (e.g., A100-80GB)
	BaseModel string `json:"base_model" yaml:"base_model"`

	// TargetModel is the model to convert to/from
	TargetModel string `json:"target_model" yaml:"target_model"`

	// ComputeRatio is the compute power ratio (target/base)
	// A ratio of 0.5 means target has half the compute power of base
	ComputeRatio float64 `json:"compute_ratio" yaml:"compute_ratio"`

	// MemoryRatio is the memory capacity ratio (target/base)
	MemoryRatio float64 `json:"memory_ratio" yaml:"memory_ratio"`
}

// ProfileConfig represents the configuration loaded from ConfigMap
type ProfileConfig struct {
	// Profiles is the list of known hardware profiles
	Profiles []HardwareProfile `json:"profiles" yaml:"profiles"`

	// BaseModel is the reference model for normalization (e.g., A100-80GB)
	BaseModel string `json:"base_model" yaml:"base_model"`
}

// NormalizedCompute represents compute power normalized to the base model
type NormalizedCompute struct {
	// BaseModel is the reference model used for normalization
	BaseModel string `json:"base_model"`

	// NormalizedTFLOPS is the compute power expressed in base model equivalents
	NormalizedTFLOPS float64 `json:"normalized_tflops"`

	// NormalizedVRAM is the memory expressed in base model equivalents
	NormalizedVRAM float64 `json:"normalized_vram"`

	// OriginalModel is the actual hardware model
	OriginalModel string `json:"original_model"`

	// OriginalVendor is the actual hardware vendor
	OriginalVendor string `json:"original_vendor"`
}

// Key returns a unique identifier for the profile (vendor/model)
func (p *HardwareProfile) Key() string {
	return p.Vendor + "/" + p.Model
}

// VRAMGiB returns the VRAM in GiB for human-readable display
func (p *HardwareProfile) VRAMGiB() float64 {
	return float64(p.VRAMBytes) / (1024 * 1024 * 1024)
}

// IsValid checks if the profile has minimum required fields
func (p *HardwareProfile) IsValid() bool {
	return p.Vendor != "" && p.Model != "" && p.VRAMBytes > 0
}

// Key returns a unique identifier for the exchange rate
func (r *ExchangeRate) Key() string {
	return r.BaseModel + "->" + r.TargetModel
}

// Inverse returns the inverse exchange rate (target -> base)
func (r *ExchangeRate) Inverse() *ExchangeRate {
	return &ExchangeRate{
		BaseModel:    r.TargetModel,
		TargetModel:  r.BaseModel,
		ComputeRatio: 1.0 / r.ComputeRatio,
		MemoryRatio:  1.0 / r.MemoryRatio,
	}
}

// BuiltinProfiles contains pre-defined hardware profiles for common models.
// These can be overridden by ConfigMap configuration.
var BuiltinProfiles = []HardwareProfile{
	// NVIDIA Data Center GPUs
	{
		Vendor:     VendorNVIDIA,
		Model:      "A100-80GB",
		FP16TFLOPS: 312,
		FP32TFLOPS: 19.5,
		VRAMBytes:  80 * 1024 * 1024 * 1024, // 80 GiB
		MemBWGBps:  2039,
		TDPWatts:   400,
	},
	{
		Vendor:     VendorNVIDIA,
		Model:      "A100-40GB",
		FP16TFLOPS: 312,
		FP32TFLOPS: 19.5,
		VRAMBytes:  40 * 1024 * 1024 * 1024, // 40 GiB
		MemBWGBps:  1555,
		TDPWatts:   400,
	},
	{
		Vendor:     VendorNVIDIA,
		Model:      "H100-80GB",
		FP16TFLOPS: 989,
		FP32TFLOPS: 67,
		VRAMBytes:  80 * 1024 * 1024 * 1024, // 80 GiB
		MemBWGBps:  3350,
		TDPWatts:   700,
	},
	{
		Vendor:     VendorNVIDIA,
		Model:      "V100-32GB",
		FP16TFLOPS: 125,
		FP32TFLOPS: 15.7,
		VRAMBytes:  32 * 1024 * 1024 * 1024, // 32 GiB
		MemBWGBps:  900,
		TDPWatts:   300,
	},
	// NVIDIA Consumer/Prosumer GPUs
	{
		Vendor:     VendorNVIDIA,
		Model:      "RTX4090",
		FP16TFLOPS: 165,
		FP32TFLOPS: 82.6,
		VRAMBytes:  24 * 1024 * 1024 * 1024, // 24 GiB
		MemBWGBps:  1008,
		TDPWatts:   450,
	},
	{
		Vendor:     VendorNVIDIA,
		Model:      "RTX3090",
		FP16TFLOPS: 71,
		FP32TFLOPS: 35.6,
		VRAMBytes:  24 * 1024 * 1024 * 1024, // 24 GiB
		MemBWGBps:  936,
		TDPWatts:   350,
	},
	// Huawei Ascend NPUs
	{
		Vendor:     VendorHuawei,
		Model:      "910B",
		FP16TFLOPS: 320,
		FP32TFLOPS: 160,
		VRAMBytes:  64 * 1024 * 1024 * 1024, // 64 GiB HBM2e
		MemBWGBps:  1600,
		TDPWatts:   400,
	},
	{
		Vendor:     VendorHuawei,
		Model:      "910A",
		FP16TFLOPS: 256,
		FP32TFLOPS: 128,
		VRAMBytes:  32 * 1024 * 1024 * 1024, // 32 GiB
		MemBWGBps:  1200,
		TDPWatts:   310,
	},
}

// DefaultBaseModel is the default reference model for normalization
const DefaultBaseModel = "A100-80GB"
