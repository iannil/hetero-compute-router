package ascend

import (
	"context"
	"fmt"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// MockDetector 模拟华为昇腾检测器，用于测试
type MockDetector struct {
	devices  []*detectors.Device
	topology *detectors.Topology
	hwType   *detectors.HardwareType
}

// MockConfig Mock 检测器配置
type MockConfig struct {
	DeviceCount int
	NPUModel    string // "Ascend 910B" 或 "Ascend 910A"
	VRAMPerNPU  uint64 // bytes
	HasHCCS     bool   // 华为高速缓存一致性系统
}

// DefaultMockConfig 默认 Mock 配置（模拟 8x Ascend 910B）
var DefaultMockConfig = MockConfig{
	DeviceCount: 8,
	NPUModel:    "Ascend 910B",
	VRAMPerNPU:  64 * 1024 * 1024 * 1024, // 64GB HBM2e
	HasHCCS:     true,
}

// computeCapByModel 根据型号返回计算能力
func computeCapByModel(model string) detectors.ComputeCapability {
	switch model {
	case "Ascend 910B":
		return detectors.ComputeCapability{
			FP16TFLOPS: 320,
			FP32TFLOPS: 160,
			Major:      9, // Ascend Da Vinci architecture
			Minor:      1,
		}
	case "Ascend 910A":
		return detectors.ComputeCapability{
			FP16TFLOPS: 256,
			FP32TFLOPS: 128,
			Major:      9,
			Minor:      0,
		}
	default:
		// 默认使用 910B 规格
		return detectors.ComputeCapability{
			FP16TFLOPS: 320,
			FP32TFLOPS: 160,
			Major:      9,
			Minor:      1,
		}
	}
}

// NewMockDetector 创建 Mock 检测器
func NewMockDetector(config *MockConfig) *MockDetector {
	if config == nil {
		config = &DefaultMockConfig
	}

	d := &MockDetector{
		hwType: &detectors.HardwareType{
			Vendor:          "huawei",
			DriverVersion:   "7.0.0", // CANN 版本
			DriverAvailable: true,
		},
	}

	computeCap := computeCapByModel(config.NPUModel)

	// 生成模拟设备
	d.devices = make([]*detectors.Device, config.DeviceCount)
	for i := 0; i < config.DeviceCount; i++ {
		d.devices[i] = &detectors.Device{
			ID:          fmt.Sprintf("npu-%d", i),
			UUID:        fmt.Sprintf("NPU-ascend-%04d-0000-0000-000000000000", i),
			Model:       config.NPUModel,
			VRAMTotal:   config.VRAMPerNPU,
			VRAMFree:    config.VRAMPerNPU,
			VRAMUsed:    0,
			PCIEBusID:   fmt.Sprintf("0000:%02x:00.0", i+1),
			Temperature: 40,
			PowerUsage:  150,
			HealthScore: 100.0,
			ECCErrors:   0,
			ComputeCap:  computeCap,
		}
	}

	// 生成模拟拓扑
	d.topology = &detectors.Topology{
		Devices: make([]detectors.TopologyDevice, config.DeviceCount),
		Links:   make([]detectors.TopologyLink, 0),
	}

	for i := 0; i < config.DeviceCount; i++ {
		d.topology.Devices[i] = detectors.TopologyDevice{
			ID:        fmt.Sprintf("npu-%d", i),
			PCIEBusID: fmt.Sprintf("0000:%02x:00.0", i+1),
		}
	}

	// 如果有 HCCS，创建全连接拓扑
	if config.HasHCCS {
		for i := 0; i < config.DeviceCount; i++ {
			for j := i + 1; j < config.DeviceCount; j++ {
				d.topology.Links = append(d.topology.Links, detectors.TopologyLink{
					SourceID:  fmt.Sprintf("npu-%d", i),
					TargetID:  fmt.Sprintf("npu-%d", j),
					Type:      detectors.LinkTypeHCCS,
					Bandwidth: 200, // GB/s, HCCS typical bandwidth
				})
			}
		}
	}

	return d
}

// Name 返回检测器名称
func (d *MockDetector) Name() string {
	return "ascend-mock"
}

// Detect 返回模拟的硬件类型
func (d *MockDetector) Detect(ctx context.Context) (*detectors.HardwareType, error) {
	return d.hwType, nil
}

// GetDevices 返回模拟的设备列表
func (d *MockDetector) GetDevices(ctx context.Context) ([]*detectors.Device, error) {
	return d.devices, nil
}

// GetTopology 返回模拟的拓扑
func (d *MockDetector) GetTopology(ctx context.Context) (*detectors.Topology, error) {
	return d.topology, nil
}

// Close 关闭检测器（Mock 无需清理）
func (d *MockDetector) Close() error {
	return nil
}

// SetDeviceUsage 设置设备的显存使用情况（用于测试）
func (d *MockDetector) SetDeviceUsage(deviceIndex int, used uint64) {
	if deviceIndex >= 0 && deviceIndex < len(d.devices) {
		d.devices[deviceIndex].VRAMUsed = used
		d.devices[deviceIndex].VRAMFree = d.devices[deviceIndex].VRAMTotal - used
	}
}

// SetDeviceHealth 设置设备的健康分（用于测试）
func (d *MockDetector) SetDeviceHealth(deviceIndex int, score float64) {
	if deviceIndex >= 0 && deviceIndex < len(d.devices) {
		d.devices[deviceIndex].HealthScore = score
	}
}

// Ensure MockDetector implements Detector interface
var _ detectors.Detector = (*MockDetector)(nil)
