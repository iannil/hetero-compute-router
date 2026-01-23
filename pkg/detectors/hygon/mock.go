package hygon

import (
	"context"
	"fmt"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// MockDetector 模拟海光 DCU 检测器，用于测试
type MockDetector struct {
	devices  []*detectors.Device
	topology *detectors.Topology
	hwType   *detectors.HardwareType
}

// MockConfig Mock 检测器配置
type MockConfig struct {
	DeviceCount int
	DCUModel    string // "DCU Z100", "DCU Z100L"
	VRAMPerDCU  uint64 // bytes
	HasxGMI     bool   // 是否有 xGMI 互联
}

// DefaultMockConfig 默认 Mock 配置（模拟 4x DCU Z100 32GB）
var DefaultMockConfig = MockConfig{
	DeviceCount: 4,
	DCUModel:    "Hygon DCU Z100",
	VRAMPerDCU:  32 * 1024 * 1024 * 1024, // 32GB
	HasxGMI:     true,
}

// NewMockDetector 创建 Mock 检测器
func NewMockDetector(config *MockConfig) *MockDetector {
	if config == nil {
		config = &DefaultMockConfig
	}

	d := &MockDetector{
		hwType: &detectors.HardwareType{
			Vendor:          "hygon",
			DriverVersion:   "5.7.0",
			DriverAvailable: true,
		},
	}

	// 根据型号确定算力
	var fp16, fp32 uint64
	switch config.DCUModel {
	case "Hygon DCU Z100L":
		fp16, fp32 = 120, 60
	default: // Z100
		fp16, fp32 = 95, 47
	}

	// 生成模拟设备
	d.devices = make([]*detectors.Device, config.DeviceCount)
	for i := 0; i < config.DeviceCount; i++ {
		d.devices[i] = &detectors.Device{
			ID:          fmt.Sprintf("dcu-%d", i),
			UUID:        fmt.Sprintf("DCU-mock-%04d-0000-0000-000000000000", i),
			Model:       config.DCUModel,
			VRAMTotal:   config.VRAMPerDCU,
			VRAMFree:    config.VRAMPerDCU,
			VRAMUsed:    0,
			PCIEBusID:   fmt.Sprintf("0000:%02x:00.0", i+1),
			Temperature: 35,
			PowerUsage:  200,
			HealthScore: 100.0,
			ECCErrors:   0,
			ComputeCap: detectors.ComputeCapability{
				FP16TFLOPS: fp16,
				FP32TFLOPS: fp32,
				Major:      9,
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
			ID:        fmt.Sprintf("dcu-%d", i),
			PCIEBusID: fmt.Sprintf("0000:%02x:00.0", i+1),
		}
	}

	// 如果有 xGMI，创建全连接拓扑
	if config.HasxGMI {
		for i := 0; i < config.DeviceCount; i++ {
			for j := i + 1; j < config.DeviceCount; j++ {
				d.topology.Links = append(d.topology.Links, detectors.TopologyLink{
					SourceID:  fmt.Sprintf("dcu-%d", i),
					TargetID:  fmt.Sprintf("dcu-%d", j),
					Type:      "xGMI",
					Bandwidth: 100, // GB/s
				})
			}
		}
	} else {
		// 仅 PCIe 连接
		for i := 0; i < config.DeviceCount; i++ {
			for j := i + 1; j < config.DeviceCount; j++ {
				d.topology.Links = append(d.topology.Links, detectors.TopologyLink{
					SourceID:  fmt.Sprintf("dcu-%d", i),
					TargetID:  fmt.Sprintf("dcu-%d", j),
					Type:      detectors.LinkTypePCIe,
					Bandwidth: 32, // GB/s
				})
			}
		}
	}

	return d
}

// Name 返回检测器名称
func (d *MockDetector) Name() string {
	return "hygon-mock"
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

// SetDeviceTemperature 设置设备的温度（用于测试）
func (d *MockDetector) SetDeviceTemperature(deviceIndex int, temp uint32) {
	if deviceIndex >= 0 && deviceIndex < len(d.devices) {
		d.devices[deviceIndex].Temperature = temp
		// 根据温度自动调整健康分
		if temp > 85 {
			d.devices[deviceIndex].HealthScore = 100.0 - float64(temp-85)*2
		} else {
			d.devices[deviceIndex].HealthScore = 100.0
		}
		if d.devices[deviceIndex].HealthScore < 0 {
			d.devices[deviceIndex].HealthScore = 0
		}
	}
}

// SetDeviceECCErrors 设置设备的 ECC 错误数（用于测试）
func (d *MockDetector) SetDeviceECCErrors(deviceIndex int, errors uint64) {
	if deviceIndex >= 0 && deviceIndex < len(d.devices) {
		d.devices[deviceIndex].ECCErrors = errors
		// 根据错误数调整健康分
		if errors > 0 {
			d.devices[deviceIndex].HealthScore = 100.0 - float64(errors)*10
			if d.devices[deviceIndex].HealthScore < 0 {
				d.devices[deviceIndex].HealthScore = 0
			}
		}
	}
}

// GetDeviceByID 根据 ID 获取设备（用于测试验证）
func (d *MockDetector) GetDeviceByID(id string) *detectors.Device {
	for _, dev := range d.devices {
		if dev.ID == id {
			return dev
		}
	}
	return nil
}

// Ensure MockDetector implements Detector interface
var _ detectors.Detector = (*MockDetector)(nil)
