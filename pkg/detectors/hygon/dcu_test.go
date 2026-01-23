package hygon

import (
	"context"
	"testing"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

func TestMockDetector_Detect(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 2,
		DCUModel:    "Hygon DCU Z100",
		VRAMPerDCU:  32 * 1024 * 1024 * 1024,
		HasxGMI:     true,
	}
	detector := NewMockDetector(config)

	ctx := context.Background()
	hwType, err := detector.Detect(ctx)

	if err != nil {
		t.Fatalf("Detect() failed: %v", err)
	}

	if hwType.Vendor != "hygon" {
		t.Errorf("Expected vendor 'hygon', got '%s'", hwType.Vendor)
	}

	if !hwType.DriverAvailable {
		t.Error("Expected DriverAvailable to be true")
	}

	if hwType.DriverVersion != "5.7.0" {
		t.Errorf("Expected driver version '5.7.0', got '%s'", hwType.DriverVersion)
	}
}

func TestMockDetector_GetDevices(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 4,
		DCUModel:    "Hygon DCU Z100",
		VRAMPerDCU:  32 * 1024 * 1024 * 1024,
		HasxGMI:     true,
	}
	detector := NewMockDetector(config)

	ctx := context.Background()
	devices, err := detector.GetDevices(ctx)

	if err != nil {
		t.Fatalf("GetDevices() failed: %v", err)
	}

	if len(devices) != config.DeviceCount {
		t.Fatalf("Expected %d devices, got %d", config.DeviceCount, len(devices))
	}

	// 检查第一个设备
	dev := devices[0]
	if dev.ID != "dcu-0" {
		t.Errorf("Expected ID 'dcu-0', got '%s'", dev.ID)
	}

	if dev.Model != config.DCUModel {
		t.Errorf("Expected model '%s', got '%s'", config.DCUModel, dev.Model)
	}

	if dev.VRAMTotal != config.VRAMPerDCU {
		t.Errorf("Expected VRAMTotal %d, got %d", config.VRAMPerDCU, dev.VRAMTotal)
	}

	if dev.ComputeCap.FP16TFLOPS != 95 {
		t.Errorf("Expected FP16TFLOPS 95, got %d", dev.ComputeCap.FP16TFLOPS)
	}

	if dev.ComputeCap.FP32TFLOPS != 47 {
		t.Errorf("Expected FP32TFLOPS 47, got %d", dev.ComputeCap.FP32TFLOPS)
	}

	if dev.HealthScore != 100.0 {
		t.Errorf("Expected HealthScore 100.0, got %f", dev.HealthScore)
	}
}

func TestMockDetector_GetDevices_Z100L(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 1,
		DCUModel:    "Hygon DCU Z100L",
		VRAMPerDCU:  64 * 1024 * 1024 * 1024,
		HasxGMI:     false,
	}
	detector := NewMockDetector(config)

	ctx := context.Background()
	devices, err := detector.GetDevices(ctx)

	if err != nil {
		t.Fatalf("GetDevices() failed: %v", err)
	}

	dev := devices[0]
	if dev.Model != "Hygon DCU Z100L" {
		t.Errorf("Expected model 'Hygon DCU Z100L', got '%s'", dev.Model)
	}

	// Z100L 有更高的算力
	if dev.ComputeCap.FP16TFLOPS != 120 {
		t.Errorf("Expected FP16TFLOPS 120 for Z100L, got %d", dev.ComputeCap.FP16TFLOPS)
	}

	if dev.ComputeCap.FP32TFLOPS != 60 {
		t.Errorf("Expected FP32TFLOPS 60 for Z100L, got %d", dev.ComputeCap.FP32TFLOPS)
	}
}

func TestMockDetector_GetTopology_WithxGMI(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 4,
		DCUModel:    "Hygon DCU Z100",
		VRAMPerDCU:  32 * 1024 * 1024 * 1024,
		HasxGMI:     true,
	}
	detector := NewMockDetector(config)

	ctx := context.Background()
	topology, err := detector.GetTopology(ctx)

	if err != nil {
		t.Fatalf("GetTopology() failed: %v", err)
	}

	if len(topology.Devices) != config.DeviceCount {
		t.Fatalf("Expected %d devices in topology, got %d", config.DeviceCount, len(topology.Devices))
	}

	// 4 个设备，xGMI 全连接应该有 6 条链路
	expectedLinks := config.DeviceCount * (config.DeviceCount - 1) / 2
	if len(topology.Links) != expectedLinks {
		t.Fatalf("Expected %d xGMI links, got %d", expectedLinks, len(topology.Links))
	}

	// 检查第一条链路
	link := topology.Links[0]
	if link.Type != "xGMI" {
		t.Errorf("Expected link type 'xGMI', got '%s'", link.Type)
	}

	if link.Bandwidth != 100 {
		t.Errorf("Expected xGMI bandwidth 100 GB/s, got %d", link.Bandwidth)
	}
}

func TestMockDetector_GetTopology_WithoutxGMI(t *testing.T) {
	config := &MockConfig{
		DeviceCount: 2,
		DCUModel:    "Hygon DCU Z100",
		VRAMPerDCU:  32 * 1024 * 1024 * 1024,
		HasxGMI:     false,
	}
	detector := NewMockDetector(config)

	ctx := context.Background()
	topology, err := detector.GetTopology(ctx)

	if err != nil {
		t.Fatalf("GetTopology() failed: %v", err)
	}

	// 2 个设备，PCIe 连接应该有 1 条链路
	if len(topology.Links) != 1 {
		t.Fatalf("Expected 1 PCIe link, got %d", len(topology.Links))
	}

	link := topology.Links[0]
	if link.Type != detectors.LinkTypePCIe {
		t.Errorf("Expected link type '%s', got '%s'", detectors.LinkTypePCIe, link.Type)
	}

	if link.Bandwidth != 32 {
		t.Errorf("Expected PCIe bandwidth 32 GB/s, got %d", link.Bandwidth)
	}
}

func TestMockDetector_DefaultConfig(t *testing.T) {
	// 测试默认配置
	detector := NewMockDetector(nil)

	ctx := context.Background()
	hwType, err := detector.Detect(ctx)

	if err != nil {
		t.Fatalf("Detect() with nil config failed: %v", err)
	}

	if hwType.Vendor != "hygon" {
		t.Errorf("Expected vendor 'hygon', got '%s'", hwType.Vendor)
	}

	devices, err := detector.GetDevices(ctx)
	if err != nil {
		t.Fatalf("GetDevices() failed: %v", err)
	}

	// 默认配置是 4 个设备
	if len(devices) != 4 {
		t.Errorf("Expected 4 devices with default config, got %d", len(devices))
	}
}

func TestMockDetector_SetDeviceUsage(t *testing.T) {
	detector := NewMockDetector(nil)

	used := uint64(16 * 1024 * 1024 * 1024) // 16GB
	detector.SetDeviceUsage(0, used)

	dev := detector.GetDeviceByID("dcu-0")
	if dev == nil {
		t.Fatal("Device dcu-0 not found")
	}

	if dev.VRAMUsed != used {
		t.Errorf("Expected VRAMUsed %d, got %d", used, dev.VRAMUsed)
	}

	if dev.VRAMFree != dev.VRAMTotal-used {
		t.Errorf("Expected VRAMFree %d, got %d", dev.VRAMTotal-used, dev.VRAMFree)
	}
}

func TestMockDetector_SetDeviceHealth(t *testing.T) {
	detector := NewMockDetector(nil)

	score := 75.0
	detector.SetDeviceHealth(0, score)

	dev := detector.GetDeviceByID("dcu-0")
	if dev == nil {
		t.Fatal("Device dcu-0 not found")
	}

	if dev.HealthScore != score {
		t.Errorf("Expected HealthScore %f, got %f", score, dev.HealthScore)
	}
}

func TestMockDetector_SetDeviceTemperature(t *testing.T) {
	detector := NewMockDetector(nil)

	// 设置高温
	temp := uint32(90)
	detector.SetDeviceTemperature(0, temp)

	dev := detector.GetDeviceByID("dcu-0")
	if dev == nil {
		t.Fatal("Device dcu-0 not found")
	}

	if dev.Temperature != temp {
		t.Errorf("Expected Temperature %d, got %d", temp, dev.Temperature)
	}

	// 健康分应该降低
	if dev.HealthScore >= 100.0 {
		t.Errorf("Expected HealthScore to be reduced due to high temperature, got %f", dev.HealthScore)
	}
}

func TestMockDetector_SetDeviceECCErrors(t *testing.T) {
	detector := NewMockDetector(nil)

	errors := uint64(2)
	detector.SetDeviceECCErrors(0, errors)

	dev := detector.GetDeviceByID("dcu-0")
	if dev == nil {
		t.Fatal("Device dcu-0 not found")
	}

	if dev.ECCErrors != errors {
		t.Errorf("Expected ECCErrors %d, got %d", errors, dev.ECCErrors)
	}

	// 健康分应该降低
	if dev.HealthScore >= 100.0 {
		t.Errorf("Expected HealthScore to be reduced due to ECC errors, got %f", dev.HealthScore)
	}
}

func TestMockDetector_Name(t *testing.T) {
	detector := NewMockDetector(nil)

	if detector.Name() != "hygon-mock" {
		t.Errorf("Expected name 'hygon-mock', got '%s'", detector.Name())
	}
}

func TestMockDetector_Close(t *testing.T) {
	detector := NewMockDetector(nil)

	if err := detector.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}
}

// Test interface compliance
func TestMockDetector_InterfaceCompliance(t *testing.T) {
	var _ detectors.Detector = (*MockDetector)(nil)
}
