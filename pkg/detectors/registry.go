package detectors

import (
	"context"
	"fmt"
	"sync"
)

// Registry 检测器注册表
type Registry struct {
	detectors map[string]Detector
	mu        sync.RWMutex
}

// NewRegistry 创建检测器注册表
func NewRegistry() *Registry {
	return &Registry{
		detectors: make(map[string]Detector),
	}
}

// Register 注册检测器
func (r *Registry) Register(detector Detector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.detectors[detector.Name()] = detector
}

// Get 获取指定名称的检测器
func (r *Registry) Get(name string) (Detector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.detectors[name]
	return d, ok
}

// List 列出所有已注册的检测器
func (r *Registry) List() []Detector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Detector, 0, len(r.detectors))
	for _, d := range r.detectors {
		result = append(result, d)
	}
	return result
}

// DetectAll 使用所有检测器检测硬件
func (r *Registry) DetectAll(ctx context.Context) ([]*DetectionResult, error) {
	r.mu.RLock()
	detectorList := make([]Detector, 0, len(r.detectors))
	for _, d := range r.detectors {
		detectorList = append(detectorList, d)
	}
	r.mu.RUnlock()

	results := make([]*DetectionResult, 0, len(detectorList))
	for _, detector := range detectorList {
		result := &DetectionResult{
			DetectorName: detector.Name(),
		}

		hwType, err := detector.Detect(ctx)
		if err != nil {
			result.Error = err
			results = append(results, result)
			continue
		}

		if !hwType.DriverAvailable {
			result.Error = fmt.Errorf("driver not available")
			results = append(results, result)
			continue
		}

		result.HardwareType = hwType

		devices, err := detector.GetDevices(ctx)
		if err != nil {
			result.Error = err
		} else {
			result.Devices = devices
		}

		topology, err := detector.GetTopology(ctx)
		if err == nil {
			result.Topology = topology
		}

		results = append(results, result)
	}

	return results, nil
}

// FindAvailable 找到第一个可用的检测器
func (r *Registry) FindAvailable(ctx context.Context) (Detector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, detector := range r.detectors {
		hwType, err := detector.Detect(ctx)
		if err == nil && hwType.DriverAvailable {
			return detector, nil
		}
	}

	return nil, fmt.Errorf("no available hardware detector found")
}

// Close 关闭所有检测器
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lastErr error
	for _, detector := range r.detectors {
		if err := detector.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// DetectionResult 检测结果
type DetectionResult struct {
	DetectorName string
	HardwareType *HardwareType
	Devices      []*Device
	Topology     *Topology
	Error        error
}

// IsAvailable 检查检测结果是否可用
func (r *DetectionResult) IsAvailable() bool {
	return r.Error == nil && r.HardwareType != nil && r.HardwareType.DriverAvailable
}

// TotalVRAM 计算总显存
func (r *DetectionResult) TotalVRAM() uint64 {
	var total uint64
	for _, d := range r.Devices {
		total += d.VRAMTotal
	}
	return total
}

// TotalFreeVRAM 计算总空闲显存
func (r *DetectionResult) TotalFreeVRAM() uint64 {
	var total uint64
	for _, d := range r.Devices {
		total += d.VRAMFree
	}
	return total
}
