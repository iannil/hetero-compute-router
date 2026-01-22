package agent

import (
	"context"
	"testing"
	"time"
)

func TestNewAgent(t *testing.T) {
	config := &Config{
		NodeName:        "test-node",
		CollectInterval: 10 * time.Second,
		ReportInterval:  30 * time.Second,
		UseMock:         true,
		MockConfig: &MockConfig{
			DeviceCount: 4,
			GPUModel:    "NVIDIA A100-SXM4-80GB",
			VRAMPerGPU:  80 * 1024 * 1024 * 1024,
			HasNVLink:   true,
		},
	}

	agent, err := New(config, nil) // nil reporter for test
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if agent == nil {
		t.Fatal("Agent should not be nil")
	}

	if agent.config.NodeName != "test-node" {
		t.Errorf("Expected node name 'test-node', got '%s'", agent.config.NodeName)
	}
}

func TestAgent_StartStop(t *testing.T) {
	config := &Config{
		NodeName:        "test-node",
		CollectInterval: 100 * time.Millisecond, // 快速采集用于测试
		ReportInterval:  200 * time.Millisecond,
		UseMock:         true,
		MockConfig: &MockConfig{
			DeviceCount: 2,
			GPUModel:    "NVIDIA A100-SXM4-80GB",
			VRAMPerGPU:  80 * 1024 * 1024 * 1024,
			HasNVLink:   true,
		},
	}

	agent, err := New(config, nil)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 启动 agent
	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// 让 agent 运行一小段时间
	time.Sleep(300 * time.Millisecond)

	// 停止 agent
	cancel()
	if err := agent.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}

func TestAgent_GetLatestMetrics(t *testing.T) {
	config := &Config{
		NodeName:        "test-node",
		CollectInterval: 100 * time.Millisecond,
		ReportInterval:  200 * time.Millisecond,
		UseMock:         true,
		MockConfig: &MockConfig{
			DeviceCount: 4,
			GPUModel:    "NVIDIA A100-SXM4-80GB",
			VRAMPerGPU:  80 * 1024 * 1024 * 1024,
			HasNVLink:   true,
		},
	}

	agent, err := New(config, nil)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 agent 让它采集一次
	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// 等待初始采集完成
	time.Sleep(150 * time.Millisecond)

	// 获取最新的采集数据
	metrics := agent.GetLatestMetrics()
	if metrics == nil {
		t.Fatal("Metrics should not be nil after collection")
	}

	if metrics.Fingerprint == nil {
		t.Error("Fingerprint should not be nil")
	}

	// 验证 VRAM 总量（4 x 80GB = 320GB）
	expectedVRAM := uint64(4 * 80 * 1024 * 1024 * 1024)
	if metrics.Fingerprint != nil && metrics.Fingerprint.VRAMTotal != expectedVRAM {
		t.Errorf("Expected total VRAM %d, got %d", expectedVRAM, metrics.Fingerprint.VRAMTotal)
	}

	// 停止 agent
	cancel()
	agent.Stop()
}

func TestAgent_GetHardwareType(t *testing.T) {
	config := &Config{
		NodeName:        "test-node",
		CollectInterval: 100 * time.Millisecond,
		ReportInterval:  200 * time.Millisecond,
		UseMock:         true,
		MockConfig: &MockConfig{
			DeviceCount: 2,
			GPUModel:    "NVIDIA A100-SXM4-80GB",
			VRAMPerGPU:  80 * 1024 * 1024 * 1024,
			HasNVLink:   true,
		},
	}

	agent, err := New(config, nil)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 agent 让它采集
	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// 等待初始采集完成
	time.Sleep(150 * time.Millisecond)

	hwType := agent.GetHardwareType()
	if hwType == nil {
		t.Fatal("HardwareType should not be nil")
	}

	if hwType.Vendor != "nvidia" {
		t.Errorf("Expected vendor 'nvidia', got '%s'", hwType.Vendor)
	}

	if !hwType.DriverAvailable {
		t.Error("Driver should be available in mock mode")
	}

	// 停止 agent
	cancel()
	agent.Stop()
}

func TestAgent_GetLatestDevices(t *testing.T) {
	config := &Config{
		NodeName:        "test-node",
		CollectInterval: 100 * time.Millisecond,
		ReportInterval:  200 * time.Millisecond,
		UseMock:         true,
		MockConfig: &MockConfig{
			DeviceCount: 4,
			GPUModel:    "NVIDIA RTX 4090",
			VRAMPerGPU:  24 * 1024 * 1024 * 1024,
			HasNVLink:   false,
		},
	}

	agent, err := New(config, nil)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 agent 让它采集
	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// 等待初始采集完成
	time.Sleep(150 * time.Millisecond)

	devices := agent.GetLatestDevices()
	if len(devices) != 4 {
		t.Errorf("Expected 4 devices, got %d", len(devices))
	}

	for i, dev := range devices {
		if dev.Model != "NVIDIA RTX 4090" {
			t.Errorf("Device %d: expected model 'NVIDIA RTX 4090', got '%s'", i, dev.Model)
		}
		if dev.VRAMTotal != 24*1024*1024*1024 {
			t.Errorf("Device %d: expected VRAM 24GB, got %d", i, dev.VRAMTotal)
		}
	}

	// 停止 agent
	cancel()
	agent.Stop()
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				NodeName:        "test-node",
				CollectInterval: 10 * time.Second,
				ReportInterval:  30 * time.Second,
				UseMock:         true,
			},
			wantErr: false,
		},
		{
			name: "empty node name",
			config: &Config{
				NodeName:        "",
				CollectInterval: 10 * time.Second,
				ReportInterval:  30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "zero collect interval uses default",
			config: &Config{
				NodeName:        "test-node",
				CollectInterval: 0,
				ReportInterval:  30 * time.Second,
				UseMock:         true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.config, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMockConfig_Defaults(t *testing.T) {
	config := &Config{
		NodeName:        "test-node",
		CollectInterval: 100 * time.Millisecond,
		ReportInterval:  200 * time.Millisecond,
		UseMock:         true,
		// MockConfig 为 nil，应该使用默认值
	}

	agent, err := New(config, nil)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 agent 让它采集
	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// 等待初始采集完成
	time.Sleep(150 * time.Millisecond)

	devices := agent.GetLatestDevices()

	// 默认 MockConfig 有 4 个设备
	if len(devices) != 4 {
		t.Errorf("Expected 4 default devices, got %d", len(devices))
	}

	// 停止 agent
	cancel()
	agent.Stop()
}

func TestAgent_WithReporter(t *testing.T) {
	reporter := NewReporter(newMockK8sClient())

	config := &Config{
		NodeName:        "test-node",
		CollectInterval: 100 * time.Millisecond,
		ReportInterval:  150 * time.Millisecond,
		UseMock:         true,
		MockConfig: &MockConfig{
			DeviceCount: 2,
			GPUModel:    "NVIDIA A100-SXM4-80GB",
			VRAMPerGPU:  80 * 1024 * 1024 * 1024,
			HasNVLink:   true,
		},
	}

	agent, err := New(config, reporter)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 启动 agent
	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// 等待几次采集和上报
	time.Sleep(400 * time.Millisecond)

	// 停止 agent
	cancel()
	if err := agent.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}

func TestAgent_Report(t *testing.T) {
	mockClient := newMockK8sClient()
	reporter := NewReporter(mockClient)

	config := &Config{
		NodeName:        "test-report-node",
		CollectInterval: 50 * time.Millisecond,
		ReportInterval:  100 * time.Millisecond,
		UseMock:         true,
		MockConfig: &MockConfig{
			DeviceCount: 2,
			GPUModel:    "NVIDIA A100-SXM4-80GB",
			VRAMPerGPU:  80 * 1024 * 1024 * 1024,
			HasNVLink:   true,
		},
	}

	agent, err := New(config, reporter)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 agent
	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// 等待上报
	time.Sleep(300 * time.Millisecond)

	// 验证 ComputeNode 已创建
	if _, ok := mockClient.computeNodes["test-report-node"]; !ok {
		t.Error("ComputeNode should be created by reporter")
	}

	// 停止 agent
	cancel()
	agent.Stop()
}

func TestInlineMockDetector_Name(t *testing.T) {
	detector := &inlineMockDetector{}
	name := detector.Name()
	if name != "nvidia-mock" {
		t.Errorf("Expected name 'nvidia-mock', got '%s'", name)
	}
}

func TestInlineNVMLDetector_Name(t *testing.T) {
	detector := &inlineNVMLDetector{}
	name := detector.Name()
	if name != "nvidia-nvml" {
		t.Errorf("Expected name 'nvidia-nvml', got '%s'", name)
	}
}

func TestInlineNVMLDetector_Detect(t *testing.T) {
	detector := &inlineNVMLDetector{}
	ctx := context.Background()

	hwType, err := detector.Detect(ctx)
	// Should return error since NVML is not available
	if err == nil {
		t.Error("Expected error from Detect when NVML not available")
	}
	if hwType == nil {
		t.Fatal("HardwareType should not be nil even on error")
	}
	if hwType.Vendor != "nvidia" {
		t.Errorf("Expected vendor 'nvidia', got '%s'", hwType.Vendor)
	}
	if hwType.DriverAvailable {
		t.Error("DriverAvailable should be false when NVML not available")
	}
}

func TestInlineNVMLDetector_GetDevices(t *testing.T) {
	detector := &inlineNVMLDetector{}
	ctx := context.Background()

	devices, err := detector.GetDevices(ctx)
	// Should return error since NVML is not available
	if err == nil {
		t.Error("Expected error from GetDevices when NVML not available")
	}
	if devices != nil {
		t.Error("Devices should be nil when error occurs")
	}
}

func TestInlineNVMLDetector_GetTopology(t *testing.T) {
	detector := &inlineNVMLDetector{}
	ctx := context.Background()

	topology, err := detector.GetTopology(ctx)
	// Should return error since NVML is not available
	if err == nil {
		t.Error("Expected error from GetTopology when NVML not available")
	}
	if topology != nil {
		t.Error("Topology should be nil when error occurs")
	}
}

func TestInlineNVMLDetector_Close(t *testing.T) {
	detector := &inlineNVMLDetector{}
	err := detector.Close()
	// Should succeed since realDetector is nil
	if err != nil {
		t.Errorf("Close should not return error, got: %v", err)
	}
}
