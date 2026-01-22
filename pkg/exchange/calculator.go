package exchange

import (
	"fmt"
	"sync"
)

// Calculator provides exchange rate calculations between hardware models.
// It enables "compute currency" conversion for cross-vendor scheduling decisions.
type Calculator struct {
	mu        sync.RWMutex
	profiles  map[string]*HardwareProfile // key: vendor/model
	baseModel string
	rates     map[string]*ExchangeRate // key: base->target
}

// NewCalculator creates a new exchange rate calculator with builtin profiles.
func NewCalculator() *Calculator {
	c := &Calculator{
		profiles:  make(map[string]*HardwareProfile),
		baseModel: DefaultBaseModel,
		rates:     make(map[string]*ExchangeRate),
	}

	// Load builtin profiles
	for i := range BuiltinProfiles {
		p := &BuiltinProfiles[i]
		c.profiles[p.Key()] = p
	}

	// Pre-calculate exchange rates from builtin profiles
	c.calculateRates()

	return c
}

// NewCalculatorWithConfig creates a calculator with custom configuration.
func NewCalculatorWithConfig(config *ProfileConfig) *Calculator {
	c := &Calculator{
		profiles:  make(map[string]*HardwareProfile),
		baseModel: DefaultBaseModel,
		rates:     make(map[string]*ExchangeRate),
	}

	if config != nil {
		if config.BaseModel != "" {
			c.baseModel = config.BaseModel
		}

		// Load profiles from config
		for i := range config.Profiles {
			p := &config.Profiles[i]
			if p.IsValid() {
				c.profiles[p.Key()] = p
			}
		}
	}

	// Fall back to builtin profiles if none provided
	if len(c.profiles) == 0 {
		for i := range BuiltinProfiles {
			p := &BuiltinProfiles[i]
			c.profiles[p.Key()] = p
		}
	}

	c.calculateRates()
	return c
}

// calculateRates pre-computes exchange rates between all profiles and the base model.
func (c *Calculator) calculateRates() {
	baseProfile := c.findProfileByModel(c.baseModel)
	if baseProfile == nil {
		return
	}

	for _, profile := range c.profiles {
		if profile.Model == c.baseModel {
			continue
		}

		rate := c.computeRate(baseProfile, profile)
		c.rates[rate.Key()] = rate
	}
}

// computeRate calculates the exchange rate between base and target profiles.
func (c *Calculator) computeRate(base, target *HardwareProfile) *ExchangeRate {
	var computeRatio, memoryRatio float64

	// Compute ratio based on FP16 TFLOPS (primary metric for AI workloads)
	if base.FP16TFLOPS > 0 {
		computeRatio = target.FP16TFLOPS / base.FP16TFLOPS
	} else if base.FP32TFLOPS > 0 {
		computeRatio = target.FP32TFLOPS / base.FP32TFLOPS
	}

	// Memory ratio based on VRAM
	if base.VRAMBytes > 0 {
		memoryRatio = float64(target.VRAMBytes) / float64(base.VRAMBytes)
	}

	return &ExchangeRate{
		BaseModel:    base.Model,
		TargetModel:  target.Model,
		ComputeRatio: computeRatio,
		MemoryRatio:  memoryRatio,
	}
}

// findProfileByModel finds a profile by model name (searches all vendors).
func (c *Calculator) findProfileByModel(model string) *HardwareProfile {
	for _, profile := range c.profiles {
		if profile.Model == model {
			return profile
		}
	}
	return nil
}

// GetProfile returns a hardware profile by vendor and model.
func (c *Calculator) GetProfile(vendor, model string) *HardwareProfile {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := vendor + "/" + model
	return c.profiles[key]
}

// GetProfileByModel returns a hardware profile by model name only.
func (c *Calculator) GetProfileByModel(model string) *HardwareProfile {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.findProfileByModel(model)
}

// GetRate returns the exchange rate between two models.
// If direct rate not found, attempts to calculate through the base model.
func (c *Calculator) GetRate(fromModel, toModel string) (*ExchangeRate, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Same model - ratio is 1:1
	if fromModel == toModel {
		return &ExchangeRate{
			BaseModel:    fromModel,
			TargetModel:  toModel,
			ComputeRatio: 1.0,
			MemoryRatio:  1.0,
		}, nil
	}

	// Direct rate from base model
	if fromModel == c.baseModel {
		key := fromModel + "->" + toModel
		if rate, ok := c.rates[key]; ok {
			return rate, nil
		}
	}

	// Inverse rate to base model
	if toModel == c.baseModel {
		key := c.baseModel + "->" + fromModel
		if rate, ok := c.rates[key]; ok {
			return rate.Inverse(), nil
		}
	}

	// Calculate through base model (from -> base -> to)
	fromProfile := c.findProfileByModel(fromModel)
	toProfile := c.findProfileByModel(toModel)
	baseProfile := c.findProfileByModel(c.baseModel)

	if fromProfile == nil {
		return nil, fmt.Errorf("unknown model: %s", fromModel)
	}
	if toProfile == nil {
		return nil, fmt.Errorf("unknown model: %s", toModel)
	}
	if baseProfile == nil {
		return nil, fmt.Errorf("base model not found: %s", c.baseModel)
	}

	// from -> base rate
	fromToBase := c.computeRate(fromProfile, baseProfile)
	// base -> to rate
	baseToTarget := c.computeRate(baseProfile, toProfile)

	// Combined rate: from -> to = (from -> base) * (base -> to)
	return &ExchangeRate{
		BaseModel:    fromModel,
		TargetModel:  toModel,
		ComputeRatio: fromToBase.ComputeRatio * baseToTarget.ComputeRatio,
		MemoryRatio:  fromToBase.MemoryRatio * baseToTarget.MemoryRatio,
	}, nil
}

// NormalizeCompute converts compute power to base model equivalents.
func (c *Calculator) NormalizeCompute(vendor, model string, deviceCount int) (*NormalizedCompute, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	profile := c.GetProfile(vendor, model)
	if profile == nil {
		// Try by model only
		profile = c.findProfileByModel(model)
	}
	if profile == nil {
		return nil, fmt.Errorf("unknown hardware: %s/%s", vendor, model)
	}

	baseProfile := c.findProfileByModel(c.baseModel)
	if baseProfile == nil {
		return nil, fmt.Errorf("base model not found: %s", c.baseModel)
	}

	// Calculate normalized values
	var normalizedTFLOPS, normalizedVRAM float64

	if baseProfile.FP16TFLOPS > 0 {
		// Total TFLOPS expressed in base model units
		totalTFLOPS := profile.FP16TFLOPS * float64(deviceCount)
		normalizedTFLOPS = totalTFLOPS / baseProfile.FP16TFLOPS
	}

	if baseProfile.VRAMBytes > 0 {
		// Total VRAM expressed in base model units
		totalVRAM := profile.VRAMBytes * uint64(deviceCount)
		normalizedVRAM = float64(totalVRAM) / float64(baseProfile.VRAMBytes)
	}

	return &NormalizedCompute{
		BaseModel:        c.baseModel,
		NormalizedTFLOPS: normalizedTFLOPS,
		NormalizedVRAM:   normalizedVRAM,
		OriginalModel:    model,
		OriginalVendor:   vendor,
	}, nil
}

// ConvertVRAM converts VRAM requirement from base model to target model.
// Returns the equivalent VRAM in bytes needed on target hardware.
func (c *Calculator) ConvertVRAM(vramBytes uint64, toModel string) (uint64, error) {
	rate, err := c.GetRate(c.baseModel, toModel)
	if err != nil {
		return 0, err
	}

	if rate.MemoryRatio == 0 {
		return 0, fmt.Errorf("invalid memory ratio for model: %s", toModel)
	}

	// Convert: target_vram = base_vram / memory_ratio
	// (if target has half the memory ratio, we need same absolute bytes)
	return vramBytes, nil
}

// ConvertCompute converts compute requirement from base model to target model.
// Returns the equivalent number of target devices needed.
func (c *Calculator) ConvertCompute(baseTFLOPS float64, toModel string) (float64, error) {
	rate, err := c.GetRate(c.baseModel, toModel)
	if err != nil {
		return 0, err
	}

	if rate.ComputeRatio == 0 {
		return 0, fmt.Errorf("invalid compute ratio for model: %s", toModel)
	}

	// target_tflops_needed = base_tflops / compute_ratio
	return baseTFLOPS / rate.ComputeRatio, nil
}

// AddProfile adds or updates a hardware profile.
func (c *Calculator) AddProfile(profile *HardwareProfile) error {
	if !profile.IsValid() {
		return fmt.Errorf("invalid profile: vendor and model required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.profiles[profile.Key()] = profile
	c.calculateRates()
	return nil
}

// SetBaseModel changes the base model for normalization.
func (c *Calculator) SetBaseModel(model string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.findProfileByModel(model) == nil {
		return fmt.Errorf("unknown model: %s", model)
	}

	c.baseModel = model
	c.calculateRates()
	return nil
}

// GetBaseModel returns the current base model.
func (c *Calculator) GetBaseModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.baseModel
}

// ListProfiles returns all registered hardware profiles.
func (c *Calculator) ListProfiles() []*HardwareProfile {
	c.mu.RLock()
	defer c.mu.RUnlock()

	profiles := make([]*HardwareProfile, 0, len(c.profiles))
	for _, p := range c.profiles {
		profiles = append(profiles, p)
	}
	return profiles
}

// ListRates returns all pre-computed exchange rates.
func (c *Calculator) ListRates() []*ExchangeRate {
	c.mu.RLock()
	defer c.mu.RUnlock()

	rates := make([]*ExchangeRate, 0, len(c.rates))
	for _, r := range c.rates {
		rates = append(rates, r)
	}
	return rates
}

// ScoreNode calculates a normalized score for a node based on its hardware.
// Higher score = more compute power in base model equivalents.
func (c *Calculator) ScoreNode(vendor, model string, deviceCount int, availableVRAM uint64) float64 {
	normalized, err := c.NormalizeCompute(vendor, model, deviceCount)
	if err != nil {
		return 0
	}

	// Combine compute and memory scores
	// Weight: 70% compute, 30% memory
	computeScore := normalized.NormalizedTFLOPS * 0.7

	baseProfile := c.findProfileByModel(c.baseModel)
	if baseProfile != nil && baseProfile.VRAMBytes > 0 {
		memoryScore := (float64(availableVRAM) / float64(baseProfile.VRAMBytes)) * 0.3
		return computeScore + memoryScore
	}

	return computeScore
}
