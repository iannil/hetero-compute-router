package nvidia

import (
	"context"
	"testing"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

func TestMockDetector_Detect(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 4,
		GPUModel:    "NVIDIA A100-SXM4-80GB",
		VRAMPerGPU:  80 * 1024 * 1024 * 1024, // 80GB
		HasNVLink:   true,
	}

	detector := NewMockDetector(config)
	defer detector.Close()

	ctx := context.Background()

	// 测试硬件类型检测
	hwType, err := detector.Detect(ctx)
	if err != nil {
		t.Fatalf("Detect() failed: %v", err)
	}

	if hwType.Vendor != "nvidia" {
		t.Errorf("Expected vendor 'nvidia', got '%s'", hwType.Vendor)
	}

	if !hwType.DriverAvailable {
		t.Error("Expected driver to be available")
	}

	if hwType.DriverVersion == "" {
		t.Error("Expected driver version to be set")
	}
}

func TestMockDetector_GetDevices(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 4,
		GPUModel:    "NVIDIA A100-SXM4-80GB",
		VRAMPerGPU:  80 * 1024 * 1024 * 1024,
		HasNVLink:   true,
	}

	detector := NewMockDetector(config)
	defer detector.Close()

	ctx := context.Background()

	devices, err := detector.GetDevices(ctx)
	if err != nil {
		t.Fatalf("GetDevices() failed: %v", err)
	}

	if len(devices) != config.DeviceCount {
		t.Errorf("Expected %d devices, got %d", config.DeviceCount, len(devices))
	}

	for i, dev := range devices {
		if dev.Model != config.GPUModel {
			t.Errorf("Device %d: expected model '%s', got '%s'", i, config.GPUModel, dev.Model)
		}

		if dev.VRAMTotal != config.VRAMPerGPU {
			t.Errorf("Device %d: expected VRAM %d, got %d", i, config.VRAMPerGPU, dev.VRAMTotal)
		}

		if dev.HealthScore != 100.0 {
			t.Errorf("Device %d: expected health score 100.0, got %f", i, dev.HealthScore)
		}

		if dev.UUID == "" {
			t.Errorf("Device %d: UUID should not be empty", i)
		}

		if dev.PCIEBusID == "" {
			t.Errorf("Device %d: PCIEBusID should not be empty", i)
		}
	}
}

func TestMockDetector_GetTopology(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 4,
		GPUModel:    "NVIDIA A100-SXM4-80GB",
		VRAMPerGPU:  80 * 1024 * 1024 * 1024,
		HasNVLink:   true,
	}

	detector := NewMockDetector(config)
	defer detector.Close()

	ctx := context.Background()

	topology, err := detector.GetTopology(ctx)
	if err != nil {
		t.Fatalf("GetTopology() failed: %v", err)
	}

	if len(topology.Devices) != config.DeviceCount {
		t.Errorf("Expected %d topology devices, got %d", config.DeviceCount, len(topology.Devices))
	}

	// 4 个设备的全连接 NVLink: C(4,2) = 6 条链接
	expectedLinks := config.DeviceCount * (config.DeviceCount - 1) / 2
	if len(topology.Links) != expectedLinks {
		t.Errorf("Expected %d NVLink links, got %d", expectedLinks, len(topology.Links))
	}

	// 验证所有链接都是 NVLink 类型
	for _, link := range topology.Links {
		if link.Type != detectors.LinkTypeNVLink {
			t.Errorf("Expected link type NVLink, got %s", link.Type)
		}

		if link.Bandwidth == 0 {
			t.Error("Expected link bandwidth > 0")
		}
	}
}

func TestMockDetector_WithoutNVLink(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 2,
		GPUModel:    "NVIDIA RTX 3090",
		VRAMPerGPU:  24 * 1024 * 1024 * 1024,
		HasNVLink:   false,
	}

	detector := NewMockDetector(config)
	defer detector.Close()

	ctx := context.Background()

	topology, err := detector.GetTopology(ctx)
	if err != nil {
		t.Fatalf("GetTopology() failed: %v", err)
	}

	// 没有 NVLink 时应该只有 PCIe 链接
	for _, link := range topology.Links {
		if link.Type != detectors.LinkTypePCIe {
			t.Errorf("Expected link type PCIe without NVLink, got %s", link.Type)
		}
	}
}

func TestMockDetector_DefaultConfig(t *testing.T) {
	// 测试 nil config 情况
	detector := NewMockDetector(nil)
	defer detector.Close()

	ctx := context.Background()

	devices, err := detector.GetDevices(ctx)
	if err != nil {
		t.Fatalf("GetDevices() failed: %v", err)
	}

	// 默认应该有 4 个设备
	if len(devices) != 4 {
		t.Errorf("Expected 4 default devices, got %d", len(devices))
	}
}

func TestMockDetector_Name(t *testing.T) {
	detector := NewMockDetector(nil)

	if detector.Name() != "nvidia-mock" {
		t.Errorf("Expected name 'nvidia-mock', got '%s'", detector.Name())
	}
}

func TestMockDetector_SetDeviceUsage(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 2,
		GPUModel:    "NVIDIA A100",
		VRAMPerGPU:  80 * 1024 * 1024 * 1024,
		HasNVLink:   true,
	}
	detector := NewMockDetector(config)
	ctx := context.Background()

	// Get initial devices
	devices, _ := detector.GetDevices(ctx)
	if devices[0].VRAMUsed != 0 {
		t.Errorf("Expected initial VRAMUsed 0, got %d", devices[0].VRAMUsed)
	}

	// Set usage for device 0
	usedBytes := uint64(20 * 1024 * 1024 * 1024)
	detector.SetDeviceUsage(0, usedBytes)

	// Verify usage was set
	devices, _ = detector.GetDevices(ctx)
	if devices[0].VRAMUsed != usedBytes {
		t.Errorf("Expected VRAMUsed %d, got %d", usedBytes, devices[0].VRAMUsed)
	}
	expectedFree := config.VRAMPerGPU - usedBytes
	if devices[0].VRAMFree != expectedFree {
		t.Errorf("Expected VRAMFree %d, got %d", expectedFree, devices[0].VRAMFree)
	}

	// Device 1 should be unchanged
	if devices[1].VRAMUsed != 0 {
		t.Errorf("Device 1 should be unchanged")
	}

	// Test out of bounds (should not panic)
	detector.SetDeviceUsage(-1, usedBytes)
	detector.SetDeviceUsage(100, usedBytes)
}

func TestMockDetector_SetDeviceHealth(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 2,
		GPUModel:    "NVIDIA A100",
		VRAMPerGPU:  80 * 1024 * 1024 * 1024,
		HasNVLink:   true,
	}
	detector := NewMockDetector(config)
	ctx := context.Background()

	// Get initial devices
	devices, _ := detector.GetDevices(ctx)
	if devices[0].HealthScore != 100.0 {
		t.Errorf("Expected initial HealthScore 100.0, got %f", devices[0].HealthScore)
	}

	// Set health for device 0
	detector.SetDeviceHealth(0, 75.0)

	// Verify health was set
	devices, _ = detector.GetDevices(ctx)
	if devices[0].HealthScore != 75.0 {
		t.Errorf("Expected HealthScore 75.0, got %f", devices[0].HealthScore)
	}

	// Device 1 should be unchanged
	if devices[1].HealthScore != 100.0 {
		t.Errorf("Device 1 should be unchanged")
	}

	// Test out of bounds (should not panic)
	detector.SetDeviceHealth(-1, 50.0)
	detector.SetDeviceHealth(100, 50.0)
}
