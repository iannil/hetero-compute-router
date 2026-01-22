package agent

import (
	"context"
	"fmt"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// createMockDetector 创建 Mock 检测器
func createMockDetector(config *MockConfig) detectors.Detector {
	return &inlineMockDetector{
		config: config,
	}
}

// createNVMLDetector 创建 NVML 检测器
func createNVMLDetector() detectors.Detector {
	// 尝试动态加载 NVML
	// 这里我们直接返回一个 NVML 检测器的实现
	return &inlineNVMLDetector{}
}

// inlineMockDetector 内联 Mock 检测器实现
type inlineMockDetector struct {
	config   *MockConfig
	devices  []*detectors.Device
	topology *detectors.Topology
	hwType   *detectors.HardwareType
	inited   bool
}

func (d *inlineMockDetector) init() {
	if d.inited {
		return
	}

	config := d.config
	if config == nil {
		config = &MockConfig{
			DeviceCount: 4,
			GPUModel:    "NVIDIA A100-SXM4-80GB",
			VRAMPerGPU:  80 * 1024 * 1024 * 1024,
			HasNVLink:   true,
		}
	}

	d.hwType = &detectors.HardwareType{
		Vendor:          "nvidia",
		DriverVersion:   "535.129.03",
		DriverAvailable: true,
	}

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

	if config.HasNVLink {
		for i := 0; i < config.DeviceCount; i++ {
			for j := i + 1; j < config.DeviceCount; j++ {
				d.topology.Links = append(d.topology.Links, detectors.TopologyLink{
					SourceID:  fmt.Sprintf("gpu-%d", i),
					TargetID:  fmt.Sprintf("gpu-%d", j),
					Type:      detectors.LinkTypeNVLink,
					Bandwidth: 300,
				})
			}
		}
	}

	d.inited = true
}

func (d *inlineMockDetector) Name() string {
	return "nvidia-mock"
}

func (d *inlineMockDetector) Detect(ctx context.Context) (*detectors.HardwareType, error) {
	d.init()
	return d.hwType, nil
}

func (d *inlineMockDetector) GetDevices(ctx context.Context) ([]*detectors.Device, error) {
	d.init()
	return d.devices, nil
}

func (d *inlineMockDetector) GetTopology(ctx context.Context) (*detectors.Topology, error) {
	d.init()
	return d.topology, nil
}

func (d *inlineMockDetector) Close() error {
	return nil
}

// inlineNVMLDetector 内联 NVML 检测器（占位符，实际调用 nvidia 包）
type inlineNVMLDetector struct {
	realDetector detectors.Detector
	initErr      error
	inited       bool
}

func (d *inlineNVMLDetector) tryInit() {
	if d.inited {
		return
	}
	d.inited = true

	// 这里应该调用真实的 NVML 初始化
	// 但为了避免循环依赖，我们使用延迟导入
	// 实际使用时，应该通过依赖注入或工厂模式来解决
	d.initErr = fmt.Errorf("NVML not available in this build")
}

func (d *inlineNVMLDetector) Name() string {
	return "nvidia-nvml"
}

func (d *inlineNVMLDetector) Detect(ctx context.Context) (*detectors.HardwareType, error) {
	d.tryInit()
	if d.initErr != nil {
		return &detectors.HardwareType{
			Vendor:          "nvidia",
			DriverAvailable: false,
		}, d.initErr
	}
	return d.realDetector.Detect(ctx)
}

func (d *inlineNVMLDetector) GetDevices(ctx context.Context) ([]*detectors.Device, error) {
	d.tryInit()
	if d.initErr != nil {
		return nil, d.initErr
	}
	return d.realDetector.GetDevices(ctx)
}

func (d *inlineNVMLDetector) GetTopology(ctx context.Context) (*detectors.Topology, error) {
	d.tryInit()
	if d.initErr != nil {
		return nil, d.initErr
	}
	return d.realDetector.GetTopology(ctx)
}

func (d *inlineNVMLDetector) Close() error {
	if d.realDetector != nil {
		return d.realDetector.Close()
	}
	return nil
}
