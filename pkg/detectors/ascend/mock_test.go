package ascend

import (
	"context"
	"testing"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

func TestMockDetector_Detect(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 8,
		NPUModel:    "Ascend 910B",
		VRAMPerNPU:  64 * 1024 * 1024 * 1024, // 64GB
		HasHCCS:     true,
	}

	detector := NewMockDetector(config)
	defer detector.Close()

	ctx := context.Background()

	// 测试硬件类型检测
	hwType, err := detector.Detect(ctx)
	if err != nil {
		t.Fatalf("Detect() failed: %v", err)
	}

	if hwType.Vendor != "huawei" {
		t.Errorf("Expected vendor 'huawei', got '%s'", hwType.Vendor)
	}

	if !hwType.DriverAvailable {
		t.Error("Expected driver to be available")
	}

	if hwType.DriverVersion == "" {
		t.Error("Expected driver version to be set")
	}

	if hwType.DriverVersion != "7.0.0" {
		t.Errorf("Expected CANN version '7.0.0', got '%s'", hwType.DriverVersion)
	}
}

func TestMockDetector_GetDevices(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 8,
		NPUModel:    "Ascend 910B",
		VRAMPerNPU:  64 * 1024 * 1024 * 1024,
		HasHCCS:     true,
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
		if dev.Model != config.NPUModel {
			t.Errorf("Device %d: expected model '%s', got '%s'", i, config.NPUModel, dev.Model)
		}

		if dev.VRAMTotal != config.VRAMPerNPU {
			t.Errorf("Device %d: expected VRAM %d, got %d", i, config.VRAMPerNPU, dev.VRAMTotal)
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

		// 验证 910B 计算能力
		if dev.ComputeCap.FP16TFLOPS != 320 {
			t.Errorf("Device %d: expected FP16 320 TFLOPS, got %d", i, dev.ComputeCap.FP16TFLOPS)
		}
		if dev.ComputeCap.FP32TFLOPS != 160 {
			t.Errorf("Device %d: expected FP32 160 TFLOPS, got %d", i, dev.ComputeCap.FP32TFLOPS)
		}
	}
}

func TestMockDetector_GetDevices_910A(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 4,
		NPUModel:    "Ascend 910A",
		VRAMPerNPU:  32 * 1024 * 1024 * 1024, // 32GB
		HasHCCS:     true,
	}

	detector := NewMockDetector(config)
	defer detector.Close()

	ctx := context.Background()

	devices, err := detector.GetDevices(ctx)
	if err != nil {
		t.Fatalf("GetDevices() failed: %v", err)
	}

	for i, dev := range devices {
		// 验证 910A 计算能力
		if dev.ComputeCap.FP16TFLOPS != 256 {
			t.Errorf("Device %d: expected FP16 256 TFLOPS, got %d", i, dev.ComputeCap.FP16TFLOPS)
		}
		if dev.ComputeCap.FP32TFLOPS != 128 {
			t.Errorf("Device %d: expected FP32 128 TFLOPS, got %d", i, dev.ComputeCap.FP32TFLOPS)
		}
	}
}

func TestMockDetector_GetTopology(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 8,
		NPUModel:    "Ascend 910B",
		VRAMPerNPU:  64 * 1024 * 1024 * 1024,
		HasHCCS:     true,
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

	// 8 个设备的全连接 HCCS: C(8,2) = 28 条链接
	expectedLinks := config.DeviceCount * (config.DeviceCount - 1) / 2
	if len(topology.Links) != expectedLinks {
		t.Errorf("Expected %d HCCS links, got %d", expectedLinks, len(topology.Links))
	}

	// 验证所有链接都是 HCCS 类型
	for _, link := range topology.Links {
		if link.Type != detectors.LinkTypeHCCS {
			t.Errorf("Expected link type HCCS, got %s", link.Type)
		}

		if link.Bandwidth == 0 {
			t.Error("Expected link bandwidth > 0")
		}

		if link.Bandwidth != 200 {
			t.Errorf("Expected HCCS bandwidth 200 GB/s, got %d", link.Bandwidth)
		}
	}
}

func TestMockDetector_WithoutHCCS(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 2,
		NPUModel:    "Ascend 910A",
		VRAMPerNPU:  32 * 1024 * 1024 * 1024,
		HasHCCS:     false,
	}

	detector := NewMockDetector(config)
	defer detector.Close()

	ctx := context.Background()

	topology, err := detector.GetTopology(ctx)
	if err != nil {
		t.Fatalf("GetTopology() failed: %v", err)
	}

	// 没有 HCCS 时应该没有链接
	if len(topology.Links) != 0 {
		t.Errorf("Expected 0 links without HCCS, got %d", len(topology.Links))
	}

	// 设备数应该正确
	if len(topology.Devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(topology.Devices))
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

	// 默认应该有 8 个设备
	if len(devices) != 8 {
		t.Errorf("Expected 8 default devices, got %d", len(devices))
	}

	// 默认型号应该是 910B
	if devices[0].Model != "Ascend 910B" {
		t.Errorf("Expected default model 'Ascend 910B', got '%s'", devices[0].Model)
	}

	// 默认显存应该是 64GB
	expectedVRAM := uint64(64 * 1024 * 1024 * 1024)
	if devices[0].VRAMTotal != expectedVRAM {
		t.Errorf("Expected default VRAM %d, got %d", expectedVRAM, devices[0].VRAMTotal)
	}
}

func TestMockDetector_Name(t *testing.T) {
	detector := NewMockDetector(nil)

	if detector.Name() != "ascend-mock" {
		t.Errorf("Expected name 'ascend-mock', got '%s'", detector.Name())
	}
}

func TestMockDetector_SetDeviceUsage(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 2,
		NPUModel:    "Ascend 910B",
		VRAMPerNPU:  64 * 1024 * 1024 * 1024,
		HasHCCS:     true,
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
	expectedFree := config.VRAMPerNPU - usedBytes
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
		NPUModel:    "Ascend 910B",
		VRAMPerNPU:  64 * 1024 * 1024 * 1024,
		HasHCCS:     true,
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

func TestMockDetector_Close(t *testing.T) {
	detector := NewMockDetector(nil)

	// Close should not return error
	err := detector.Close()
	if err != nil {
		t.Errorf("Close() should not return error, got: %v", err)
	}
}

func TestComputeCapByModel(t *testing.T) {
	tests := []struct {
		model      string
		fp16       uint64
		fp32       uint64
		majorMinor string
	}{
		{"Ascend 910B", 320, 160, "9.1"},
		{"Ascend 910A", 256, 128, "9.0"},
		{"Unknown Model", 320, 160, "9.1"}, // defaults to 910B
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			cap := computeCapByModel(tt.model)
			if cap.FP16TFLOPS != tt.fp16 {
				t.Errorf("Expected FP16 %d, got %d", tt.fp16, cap.FP16TFLOPS)
			}
			if cap.FP32TFLOPS != tt.fp32 {
				t.Errorf("Expected FP32 %d, got %d", tt.fp32, cap.FP32TFLOPS)
			}
		})
	}
}
