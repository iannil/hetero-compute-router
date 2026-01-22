package nvidia

import (
	"context"
	"fmt"
	"sync"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// NVMLDetector NVIDIA 硬件检测器，基于 NVML 库
type NVMLDetector struct {
	initialized bool
	mu          sync.Mutex
}

// NewNVMLDetector 创建 NVIDIA 检测器
func NewNVMLDetector() *NVMLDetector {
	return &NVMLDetector{}
}

// Name 返回检测器名称
func (d *NVMLDetector) Name() string {
	return "nvidia-nvml"
}

// init 初始化 NVML 库
func (d *NVMLDetector) init() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.initialized {
		return nil
	}

	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
	}

	d.initialized = true
	return nil
}

// Detect 检测 NVIDIA 硬件类型和可用性
func (d *NVMLDetector) Detect(ctx context.Context) (*detectors.HardwareType, error) {
	if err := d.init(); err != nil {
		return &detectors.HardwareType{
			Vendor:          "nvidia",
			DriverAvailable: false,
		}, err
	}

	driverVersion, ret := nvml.SystemGetDriverVersion()
	if ret != nvml.SUCCESS {
		return &detectors.HardwareType{
			Vendor:          "nvidia",
			DriverAvailable: true,
		}, nil
	}

	return &detectors.HardwareType{
		Vendor:          "nvidia",
		DriverVersion:   driverVersion,
		DriverAvailable: true,
	}, nil
}

// GetDevices 获取 NVIDIA 设备列表
func (d *NVMLDetector) GetDevices(ctx context.Context) ([]*detectors.Device, error) {
	if err := d.init(); err != nil {
		return nil, err
	}

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get device count: %v", nvml.ErrorString(ret))
	}

	devices := make([]*detectors.Device, 0, count)
	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			continue
		}

		dev, err := d.getDeviceInfo(device, i)
		if err != nil {
			continue
		}
		devices = append(devices, dev)
	}

	return devices, nil
}

// getDeviceInfo 获取单个设备的详细信息
func (d *NVMLDetector) getDeviceInfo(device nvml.Device, index int) (*detectors.Device, error) {
	dev := &detectors.Device{
		ID:          fmt.Sprintf("gpu-%d", index),
		HealthScore: 100.0, // 默认满分
	}

	// UUID
	if uuid, ret := device.GetUUID(); ret == nvml.SUCCESS {
		dev.UUID = uuid
	}

	// 型号名称
	if name, ret := device.GetName(); ret == nvml.SUCCESS {
		dev.Model = name
	}

	// 显存信息
	if memInfo, ret := device.GetMemoryInfo(); ret == nvml.SUCCESS {
		dev.VRAMTotal = memInfo.Total
		dev.VRAMFree = memInfo.Free
		dev.VRAMUsed = memInfo.Used
	}

	// PCI 总线 ID
	if pciInfo, ret := device.GetPciInfo(); ret == nvml.SUCCESS {
		dev.PCIEBusID = fmt.Sprintf("%04x:%02x:%02x.0",
			pciInfo.Domain, pciInfo.Bus, pciInfo.Device)
	}

	// 计算能力
	if major, minor, ret := device.GetCudaComputeCapability(); ret == nvml.SUCCESS {
		dev.ComputeCap.Major = major
		dev.ComputeCap.Minor = minor
		// 根据计算能力估算 TFLOPS（简化计算）
		dev.ComputeCap.FP16TFLOPS, dev.ComputeCap.FP32TFLOPS = estimateTFLOPS(dev.Model, major, minor)
	}

	// 温度
	if temp, ret := device.GetTemperature(nvml.TEMPERATURE_GPU); ret == nvml.SUCCESS {
		dev.Temperature = temp
		// 温度过高降低健康分
		if temp > 85 {
			dev.HealthScore -= float64(temp-85) * 2
		}
	}

	// 功耗
	if power, ret := device.GetPowerUsage(); ret == nvml.SUCCESS {
		dev.PowerUsage = power / 1000 // mW to W
	}

	// ECC 错误
	if eccErrors, ret := device.GetTotalEccErrors(nvml.MEMORY_ERROR_TYPE_UNCORRECTED, nvml.VOLATILE_ECC); ret == nvml.SUCCESS {
		dev.ECCErrors = eccErrors
		// ECC 错误降低健康分
		if eccErrors > 0 {
			dev.HealthScore -= float64(eccErrors) * 10
		}
	}

	// 确保健康分不低于 0
	if dev.HealthScore < 0 {
		dev.HealthScore = 0
	}

	return dev, nil
}

// GetTopology 获取设备间拓扑关系
func (d *NVMLDetector) GetTopology(ctx context.Context) (*detectors.Topology, error) {
	if err := d.init(); err != nil {
		return nil, err
	}

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get device count: %v", nvml.ErrorString(ret))
	}

	topology := &detectors.Topology{
		Devices: make([]detectors.TopologyDevice, 0, count),
		Links:   make([]detectors.TopologyLink, 0),
	}

	// 收集设备信息
	handles := make([]nvml.Device, 0, count)
	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			continue
		}
		handles = append(handles, device)

		id := fmt.Sprintf("gpu-%d", i)
		pcieBusID := ""
		if pciInfo, ret := device.GetPciInfo(); ret == nvml.SUCCESS {
			pcieBusID = fmt.Sprintf("%04x:%02x:%02x.0",
				pciInfo.Domain, pciInfo.Bus, pciInfo.Device)
		}

		topology.Devices = append(topology.Devices, detectors.TopologyDevice{
			ID:        id,
			PCIEBusID: pcieBusID,
		})
	}

	// 检测设备间连接
	for i := 0; i < len(handles); i++ {
		for j := i + 1; j < len(handles); j++ {
			linkType, bandwidth := d.detectLink(handles[i], handles[j])
			if linkType != detectors.LinkTypeUnknown {
				topology.Links = append(topology.Links, detectors.TopologyLink{
					SourceID:  fmt.Sprintf("gpu-%d", i),
					TargetID:  fmt.Sprintf("gpu-%d", j),
					Type:      linkType,
					Bandwidth: bandwidth,
				})
			}
		}
	}

	return topology, nil
}

// detectLink 检测两个设备间的连接类型
func (d *NVMLDetector) detectLink(dev1, dev2 nvml.Device) (detectors.LinkType, uint64) {
	p2pStatus, ret := dev1.GetP2PStatus(dev2, nvml.P2P_CAPS_INDEX_NVLINK)
	if ret == nvml.SUCCESS && p2pStatus == nvml.P2P_STATUS_OK {
		// NVLink 连接，带宽约 300 GB/s (NVLink 3.0)
		return detectors.LinkTypeNVLink, 300
	}

	// 检查是否可以通过 PCIe P2P
	p2pStatus, ret = dev1.GetP2PStatus(dev2, nvml.P2P_CAPS_INDEX_READ)
	if ret == nvml.SUCCESS && p2pStatus == nvml.P2P_STATUS_OK {
		// PCIe 连接，带宽约 32 GB/s (PCIe 4.0 x16)
		return detectors.LinkTypePCIe, 32
	}

	return detectors.LinkTypeUnknown, 0
}

// Close 关闭检测器
func (d *NVMLDetector) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.initialized {
		return nil
	}

	ret := nvml.Shutdown()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to shutdown NVML: %v", nvml.ErrorString(ret))
	}

	d.initialized = false
	return nil
}

// estimateTFLOPS 根据 GPU 型号和计算能力估算 TFLOPS
func estimateTFLOPS(model string, major, minor int) (fp16, fp32 uint64) {
	// 基于已知 GPU 型号的性能数据
	// 这是一个简化的查找表，实际应该从数据库或配置文件加载
	knownModels := map[string][2]uint64{
		"NVIDIA A100-SXM4-80GB":   {312, 19},
		"NVIDIA A100-SXM4-40GB":   {312, 19},
		"NVIDIA A100-PCIE-80GB":   {312, 19},
		"NVIDIA A100-PCIE-40GB":   {312, 19},
		"NVIDIA H100 PCIe":        {756, 51},
		"NVIDIA H100 SXM5":        {989, 67},
		"NVIDIA A800-SXM4-80GB":   {312, 19},
		"NVIDIA V100-SXM2-32GB":   {125, 15},
		"NVIDIA V100-SXM2-16GB":   {125, 15},
		"NVIDIA Tesla T4":         {65, 8},
		"NVIDIA GeForce RTX 4090": {330, 83},
		"NVIDIA GeForce RTX 3090": {142, 35},
	}

	if tflops, ok := knownModels[model]; ok {
		return tflops[0], tflops[1]
	}

	// 根据计算能力粗略估算
	// Ampere (8.x): ~20 TFLOPS FP32
	// Hopper (9.x): ~60 TFLOPS FP32
	switch major {
	case 9:
		return 500, 60
	case 8:
		return 150, 20
	case 7:
		return 100, 14
	default:
		return 50, 10
	}
}

// Ensure NVMLDetector implements Detector interface
var _ detectors.Detector = (*NVMLDetector)(nil)
