package nvidia

import (
	"context"
	"fmt"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// MockDetector 模拟 NVIDIA 检测器，用于测试
type MockDetector struct {
	devices  []*detectors.Device
	topology *detectors.Topology
	hwType   *detectors.HardwareType
}

// MockConfig Mock 检测器配置
type MockConfig struct {
	DeviceCount int
	GPUModel    string
	VRAMPerGPU  uint64 // bytes
	HasNVLink   bool
}

// DefaultMockConfig 默认 Mock 配置（模拟 4x A100-80GB）
var DefaultMockConfig = MockConfig{
	DeviceCount: 4,
	GPUModel:    "NVIDIA A100-SXM4-80GB",
	VRAMPerGPU:  80 * 1024 * 1024 * 1024, // 80GB
	HasNVLink:   true,
}

// NewMockDetector 创建 Mock 检测器
func NewMockDetector(config *MockConfig) *MockDetector {
	if config == nil {
		config = &DefaultMockConfig
	}

	d := &MockDetector{
		hwType: &detectors.HardwareType{
			Vendor:          "nvidia",
			DriverVersion:   "535.129.03",
			DriverAvailable: true,
		},
	}

	// 生成模拟设备
	d.devices = make([]*detectors.Device, config.DeviceCount)
	for i := 0; i < config.DeviceCount; i++ {
		d.devices[i] = &detectors.Device{
			ID:          fmt.Sprintf("gpu-%d", i),
			UUID:        fmt.Sprintf("GPU-mock-%04d-0000-0000-000000000000", i),
			Model:       config.GPUModel,
			VRAMTotal:   config.VRAMPerGPU,
			VRAMFree:    config.VRAMPerGPU,
			VRAMUsed:    0,
			PCIEBusID:   fmt.Sprintf("0000:%02x:00.0", i+1),
			Temperature: 35,
			PowerUsage:  100,
			HealthScore: 100.0,
			ECCErrors:   0,
			ComputeCap: detectors.ComputeCapability{
				FP16TFLOPS: 312,
				FP32TFLOPS: 19,
				Major:      8,
				Minor:      0,
			},
		}
	}

	// 生成模拟拓扑
	d.topology = &detectors.Topology{
		Devices: make([]detectors.TopologyDevice, config.DeviceCount),
		Links:   make([]detectors.TopologyLink, 0),
	}

	for i := 0; i < config.DeviceCount; i++ {
		d.topology.Devices[i] = detectors.TopologyDevice{
			ID:        fmt.Sprintf("gpu-%d", i),
			PCIEBusID: fmt.Sprintf("0000:%02x:00.0", i+1),
		}
	}

	// 如果有 NVLink，创建全连接拓扑
	if config.HasNVLink {
		for i := 0; i < config.DeviceCount; i++ {
			for j := i + 1; j < config.DeviceCount; j++ {
				d.topology.Links = append(d.topology.Links, detectors.TopologyLink{
					SourceID:  fmt.Sprintf("gpu-%d", i),
					TargetID:  fmt.Sprintf("gpu-%d", j),
					Type:      detectors.LinkTypeNVLink,
					Bandwidth: 300, // GB/s
				})
			}
		}
	}

	return d
}

// Name 返回检测器名称
func (d *MockDetector) Name() string {
	return "nvidia-mock"
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
