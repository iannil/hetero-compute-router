package suite

import (
	"context"
	"testing"
	"time"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
	nvidia "github.com/zrs-products/hetero-compute-router/pkg/detectors/nvidia"
	hygon "github.com/zrs-products/hetero-compute-router/pkg/detectors/hygon"
)

// TestDetectorRegistration 测试检测器注册
func TestDetectorRegistration(t *testing.T) {
	registry := detectors.NewRegistry()

	// 注册 NVIDIA 检测器
	nvmlDetector := nvidia.NewNVMLDetector()
	registry.Register(nvmlDetector)

	// 注册海光检测器
	hygonDetector := hygon.NewDCUDetector()
	registry.Register(hygonDetector)

	// 验证注册
	all := registry.List()
	if len(all) != 2 {
		t.Errorf("Expected 2 detectors, got %d", len(all))
	}
}

// TestDetectorDiscovery 测试检测器发现
func TestDetectorDiscovery(t *testing.T) {
	registry := detectors.NewRegistry()

	// 注册 mock 检测器用于测试
	nvidiaMock := nvidia.NewMockDetector(&nvidia.MockConfig{
		DeviceCount: 2,
		GPUModel:    "NVIDIA A100-SXM4-80GB",
		VRAMPerGPU:  80 * 1024 * 1024 * 1024,
		HasNVLink:   true,
	})
	registry.Register(nvidiaMock)

	hygonMock := hygon.NewMockDetector(&hygon.MockConfig{
		DeviceCount: 4,
		DCUModel:    "Hygon DCU Z100",
		VRAMPerDCU:  32 * 1024 * 1024 * 1024,
		HasxGMI:     true,
	})
	registry.Register(hygonMock)

	// FindAvailable 应该返回第一个注册的检测器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	detector, err := registry.FindAvailable(ctx)
	if err != nil {
		t.Fatalf("FindAvailable failed: %v", err)
	}

	if detector.Name() != nvidiaMock.Name() {
		t.Errorf("Expected %s, got %s", nvidiaMock.Name(), detector.Name())
	}
}

// TestDetectorDetectAll 测试所有检测器
func TestDetectorDetectAll(t *testing.T) {
	registry := detectors.NewRegistry()

	// 注册不同的 mock 检测器
	registry.Register(nvidia.NewMockDetector(&nvidia.MockConfig{
		DeviceCount: 2,
		GPUModel:    "NVIDIA A100-SXM4-80GB",
		VRAMPerGPU:  80 * 1024 * 1024 * 1024,
		HasNVLink:   true,
	}))

	registry.Register(hygon.NewMockDetector(&hygon.MockConfig{
		DeviceCount: 4,
		DCUModel:    "Hygon DCU Z100",
		VRAMPerDCU:  32 * 1024 * 1024 * 1024,
		HasxGMI:     true,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := registry.DetectAll(ctx)
	if err != nil {
		t.Fatalf("DetectAll failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// 验证 NVIDIA 结果
	nvidiaResult := results[0]
	if !nvidiaResult.IsAvailable() {
		t.Error("NVIDIA detector should be available")
	}
	if len(nvidiaResult.Devices) != 2 {
		t.Errorf("Expected 2 NVIDIA devices, got %d", len(nvidiaResult.Devices))
	}

	// 验证海光结果
	hygonResult := results[1]
	if !hygonResult.IsAvailable() {
		t.Error("Hygon detector should be available")
	}
	if len(hygonResult.Devices) != 4 {
		t.Errorf("Expected 4 Hygon devices, got %d", len(hygonResult.Devices))
	}
}

// TestDetectorTopology 测试拓扑检测
func TestDetectorTopology(t *testing.T) {
	tests := []struct {
		name       string
		detector   detectors.Detector
		linkCount  int
		linkType   string
		bandwidth  uint64
	}{
		{
			name: "NVIDIA with NVLink",
			detector: nvidia.NewMockDetector(&nvidia.MockConfig{
				DeviceCount: 4,
				GPUModel:    "NVIDIA A100-SXM4-80GB",
				VRAMPerGPU:  80 * 1024 * 1024 * 1024,
				HasNVLink:   true,
			}),
			linkCount: 6, // 4个设备全连接
			linkType:   "NVLink",
			bandwidth:  300,
		},
		{
			name: "Hygon with xGMI",
			detector: hygon.NewMockDetector(&hygon.MockConfig{
				DeviceCount: 4,
				DCUModel:    "Hygon DCU Z100",
				VRAMPerDCU:  32 * 1024 * 1024 * 1024,
				HasxGMI:     true,
			}),
			linkCount: 6,
			linkType:   "xGMI",
			bandwidth:  100,
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topology, err := tt.detector.GetTopology(ctx)
			if err != nil {
				t.Fatalf("GetTopology failed: %v", err)
			}

			if len(topology.Links) != tt.linkCount {
				t.Errorf("Expected %d links, got %d", tt.linkCount, len(topology.Links))
			}

			if len(topology.Links) > 0 {
				link := topology.Links[0]
				if string(link.Type) != tt.linkType {
					t.Errorf("Expected link type %s, got %s", tt.linkType, link.Type)
				}
				if link.Bandwidth != tt.bandwidth {
					t.Errorf("Expected bandwidth %d, got %d", tt.bandwidth, link.Bandwidth)
				}
			}
		})
	}
}

// TestDetectorDeviceProperties 测试设备属性
func TestDetectorDeviceProperties(t *testing.T) {
	tests := []struct {
		name              string
		detector          detectors.Detector
		expectedModel     string
		expectedVRAM      uint64
		expectedFP16      uint64
		expectedFP32      uint64
	}{
		{
			name: "NVIDIA A100",
			detector: nvidia.NewMockDetector(&nvidia.MockConfig{
				DeviceCount: 1,
				GPUModel:    "NVIDIA A100-SXM4-80GB",
				VRAMPerGPU:  80 * 1024 * 1024 * 1024,
				HasNVLink:   true,
			}),
			expectedModel: "NVIDIA A100-SXM4-80GB",
			expectedVRAM:  80 * 1024 * 1024 * 1024,
			expectedFP16:  312,
			expectedFP32:  19,
		},
		{
			name: "Hygon DCU Z100",
			detector: hygon.NewMockDetector(&hygon.MockConfig{
				DeviceCount: 1,
				DCUModel:    "Hygon DCU Z100",
				VRAMPerDCU:  32 * 1024 * 1024 * 1024,
				HasxGMI:     true,
			}),
			expectedModel: "Hygon DCU Z100",
			expectedVRAM:  32 * 1024 * 1024 * 1024,
			expectedFP16:  95,
			expectedFP32:  47,
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devices, err := tt.detector.GetDevices(ctx)
			if err != nil {
				t.Fatalf("GetDevices failed: %v", err)
			}

			if len(devices) != 1 {
				t.Fatalf("Expected 1 device, got %d", len(devices))
			}

			dev := devices[0]

			if dev.Model != tt.expectedModel {
				t.Errorf("Expected model %s, got %s", tt.expectedModel, dev.Model)
			}

			if dev.VRAMTotal != tt.expectedVRAM {
				t.Errorf("Expected VRAM %d, got %d", tt.expectedVRAM, dev.VRAMTotal)
			}

			if dev.ComputeCap.FP16TFLOPS != tt.expectedFP16 {
				t.Errorf("Expected FP16 %d, got %d", tt.expectedFP16, dev.ComputeCap.FP16TFLOPS)
			}

			if dev.ComputeCap.FP32TFLOPS != tt.expectedFP32 {
				t.Errorf("Expected FP32 %d, got %d", tt.expectedFP32, dev.ComputeCap.FP32TFLOPS)
			}
		})
	}
}
