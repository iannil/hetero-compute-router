package detectors

import (
	"context"
	"errors"
	"testing"
)

// testMockDetector 用于测试的 Mock 检测器
type testMockDetector struct {
	name            string
	driverAvailable bool
	detectErr       error
	devicesErr      error
	topologyErr     error
	devices         []*Device
	topology        *Topology
}

func (d *testMockDetector) Name() string {
	return d.name
}

func (d *testMockDetector) Detect(ctx context.Context) (*HardwareType, error) {
	if d.detectErr != nil {
		return nil, d.detectErr
	}
	return &HardwareType{
		Vendor:          "test",
		DriverAvailable: d.driverAvailable,
		DriverVersion:   "1.0",
	}, nil
}

func (d *testMockDetector) GetDevices(ctx context.Context) ([]*Device, error) {
	if d.devicesErr != nil {
		return nil, d.devicesErr
	}
	if d.devices != nil {
		return d.devices, nil
	}
	return []*Device{
		{
			ID:        "gpu-0",
			UUID:      "GPU-TEST-0",
			Model:     "Test GPU",
			VRAMTotal: 16 * 1024 * 1024 * 1024,
			VRAMFree:  12 * 1024 * 1024 * 1024,
		},
	}, nil
}

func (d *testMockDetector) GetTopology(ctx context.Context) (*Topology, error) {
	if d.topologyErr != nil {
		return nil, d.topologyErr
	}
	if d.topology != nil {
		return d.topology, nil
	}
	return &Topology{
		Devices: []TopologyDevice{{ID: "gpu-0"}},
	}, nil
}

func (d *testMockDetector) Close() error {
	return nil
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry should return non-nil")
	}

	if len(r.List()) != 0 {
		t.Errorf("New registry should be empty")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	mock := &testMockDetector{name: "test-detector", driverAvailable: true}

	r.Register(mock)

	if len(r.List()) != 1 {
		t.Errorf("Registry should have 1 detector")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	mock := &testMockDetector{name: "test-detector", driverAvailable: true}
	r.Register(mock)

	// 测试获取存在的检测器
	d, ok := r.Get("test-detector")
	if !ok {
		t.Error("Should find registered detector")
	}
	if d.Name() != "test-detector" {
		t.Errorf("Expected name 'test-detector', got '%s'", d.Name())
	}

	// 测试获取不存在的检测器
	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("Should not find unregistered detector")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	mock1 := &testMockDetector{name: "detector-1", driverAvailable: true}
	mock2 := &testMockDetector{name: "detector-2", driverAvailable: true}

	r.Register(mock1)
	r.Register(mock2)

	list := r.List()
	if len(list) != 2 {
		t.Errorf("Expected 2 detectors, got %d", len(list))
	}
}

func TestRegistry_FindAvailable(t *testing.T) {
	ctx := context.Background()

	// 测试找到可用检测器
	r := NewRegistry()
	unavailable := &testMockDetector{name: "unavailable", driverAvailable: false}
	available := &testMockDetector{name: "available", driverAvailable: true}

	r.Register(unavailable)
	r.Register(available)

	d, err := r.FindAvailable(ctx)
	if err != nil {
		t.Fatalf("FindAvailable failed: %v", err)
	}
	if d.Name() != "available" {
		t.Errorf("Expected 'available' detector, got '%s'", d.Name())
	}

	// 测试无可用检测器
	r2 := NewRegistry()
	r2.Register(&testMockDetector{name: "test", driverAvailable: false})

	_, err = r2.FindAvailable(ctx)
	if err == nil {
		t.Error("Should return error when no available detector")
	}

	// 测试检测出错
	r3 := NewRegistry()
	r3.Register(&testMockDetector{name: "error", detectErr: errors.New("detect error")})

	_, err = r3.FindAvailable(ctx)
	if err == nil {
		t.Error("Should return error when detection fails")
	}
}

func TestRegistry_DetectAll(t *testing.T) {
	ctx := context.Background()
	r := NewRegistry()

	mock1 := &testMockDetector{
		name:            "good-detector",
		driverAvailable: true,
		devices: []*Device{
			{ID: "gpu-0", VRAMTotal: 16 * 1024 * 1024 * 1024, VRAMFree: 12 * 1024 * 1024 * 1024},
		},
	}
	mock2 := &testMockDetector{
		name:            "unavailable-detector",
		driverAvailable: false,
	}
	mock3 := &testMockDetector{
		name:      "error-detector",
		detectErr: errors.New("detection failed"),
	}

	r.Register(mock1)
	r.Register(mock2)
	r.Register(mock3)

	results, err := r.DetectAll(ctx)
	if err != nil {
		t.Fatalf("DetectAll failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// 验证结果
	for _, result := range results {
		switch result.DetectorName {
		case "good-detector":
			if !result.IsAvailable() {
				t.Error("good-detector should be available")
			}
			if len(result.Devices) != 1 {
				t.Errorf("good-detector should have 1 device")
			}
		case "unavailable-detector":
			if result.IsAvailable() {
				t.Error("unavailable-detector should not be available")
			}
		case "error-detector":
			if result.IsAvailable() {
				t.Error("error-detector should not be available")
			}
			if result.Error == nil {
				t.Error("error-detector should have error")
			}
		}
	}
}

func TestRegistry_DetectAll_DeviceError(t *testing.T) {
	ctx := context.Background()
	r := NewRegistry()

	mock := &testMockDetector{
		name:            "device-error",
		driverAvailable: true,
		devicesErr:      errors.New("device error"),
	}
	r.Register(mock)

	results, err := r.DetectAll(ctx)
	if err != nil {
		t.Fatalf("DetectAll failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result")
	}

	if results[0].Error == nil {
		t.Error("Should have device error")
	}
}

func TestRegistry_Close(t *testing.T) {
	r := NewRegistry()
	mock := &testMockDetector{name: "test", driverAvailable: true}
	r.Register(mock)

	err := r.Close()
	if err != nil {
		t.Errorf("Close should not error: %v", err)
	}
}

func TestDetectionResult_IsAvailable(t *testing.T) {
	tests := []struct {
		name     string
		result   *DetectionResult
		expected bool
	}{
		{
			name: "available",
			result: &DetectionResult{
				HardwareType: &HardwareType{DriverAvailable: true},
			},
			expected: true,
		},
		{
			name: "driver not available",
			result: &DetectionResult{
				HardwareType: &HardwareType{DriverAvailable: false},
			},
			expected: false,
		},
		{
			name: "has error",
			result: &DetectionResult{
				HardwareType: &HardwareType{DriverAvailable: true},
				Error:        errors.New("some error"),
			},
			expected: false,
		},
		{
			name:     "nil hardware type",
			result:   &DetectionResult{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.IsAvailable() != tt.expected {
				t.Errorf("IsAvailable() = %v, want %v", tt.result.IsAvailable(), tt.expected)
			}
		})
	}
}

func TestDetectionResult_TotalVRAM(t *testing.T) {
	result := &DetectionResult{
		Devices: []*Device{
			{VRAMTotal: 16 * 1024 * 1024 * 1024},
			{VRAMTotal: 24 * 1024 * 1024 * 1024},
		},
	}

	expected := uint64(40 * 1024 * 1024 * 1024)
	if result.TotalVRAM() != expected {
		t.Errorf("TotalVRAM() = %d, want %d", result.TotalVRAM(), expected)
	}
}

func TestDetectionResult_TotalFreeVRAM(t *testing.T) {
	result := &DetectionResult{
		Devices: []*Device{
			{VRAMFree: 12 * 1024 * 1024 * 1024},
			{VRAMFree: 20 * 1024 * 1024 * 1024},
		},
	}

	expected := uint64(32 * 1024 * 1024 * 1024)
	if result.TotalFreeVRAM() != expected {
		t.Errorf("TotalFreeVRAM() = %d, want %d", result.TotalFreeVRAM(), expected)
	}
}

func TestDetectionResult_EmptyDevices(t *testing.T) {
	result := &DetectionResult{}

	if result.TotalVRAM() != 0 {
		t.Errorf("TotalVRAM() should be 0 for empty devices")
	}

	if result.TotalFreeVRAM() != 0 {
		t.Errorf("TotalFreeVRAM() should be 0 for empty devices")
	}
}
