package hygon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// DCUDetector 海光 DCU 硬件检测器
// 海光 DCU 使用 ROCm/KFD 接口，兼容 AMD GPU 生态
type DCUDetector struct {
	initialized bool
	mu          sync.Mutex
	deviceCount int
}

// NewDCUDetector 创建海光 DCU 检测器
func NewDCUDetector() *DCUDetector {
	return &DCUDetector{}
}

// Name 返回检测器名称
func (d *DCUDetector) Name() string {
	return "hygon-dcu"
}

// init 初始化检测器
func (d *DCUDetector) init() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.initialized {
		return nil
	}

	// 检查 KFD 设备是否存在
	if _, err := os.Stat("/dev/kfd"); os.IsNotExist(err) {
		return fmt.Errorf("KFD device not found: /dev/kfd does not exist")
	}

	// 检查拓扑目录是否存在
	topologyPath := "/sys/class/kfd/kfd/topology/nodes"
	if _, err := os.Stat(topologyPath); os.IsNotExist(err) {
		return fmt.Errorf("KFD topology not found: %s does not exist", topologyPath)
	}

	// 获取设备数量
	entries, err := os.ReadDir(topologyPath)
	if err != nil {
		return fmt.Errorf("failed to read topology nodes: %w", err)
	}

	d.deviceCount = 0
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "node") {
			d.deviceCount++
		}
	}

	if d.deviceCount == 0 {
		return fmt.Errorf("no DCU devices found")
	}

	d.initialized = true
	return nil
}

// Detect 检测海光 DCU 硬件类型和可用性
func (d *DCUDetector) Detect(ctx context.Context) (*detectors.HardwareType, error) {
	if err := d.init(); err != nil {
		return &detectors.HardwareType{
			Vendor:          "hygon",
			DriverAvailable: false,
		}, err
	}

	// 尝试读取驱动版本
	driverVersion := d.getDriverVersion()

	return &detectors.HardwareType{
		Vendor:          "hygon",
		DriverVersion:   driverVersion,
		DriverAvailable: true,
	}, nil
}

// getDriverVersion 获取驱动版本
func (d *DCUDetector) getDriverVersion() string {
	// 尝试从 /sys/module/amdgpu/version 读取
	if data, err := os.ReadFile("/sys/module/amdgpu/version"); err == nil {
		return strings.TrimSpace(string(data))
	}

	// 尝试从 /proc/driver/amd/amdgpu/version 读取
	if data, err := os.ReadFile("/proc/driver/amd/amdgpu/version"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.Contains(line, "version") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					return parts[1]
				}
			}
		}
	}

	return "unknown"
}

// GetDevices 获取海光 DCU 设备列表
func (d *DCUDetector) GetDevices(ctx context.Context) ([]*detectors.Device, error) {
	if err := d.init(); err != nil {
		return nil, err
	}

	devices := make([]*detectors.Device, 0, d.deviceCount)

	for i := 0; i < d.deviceCount; i++ {
		nodePath := fmt.Sprintf("/sys/class/kfd/kfd/topology/nodes/node%d", i)
		dev, err := d.getDeviceInfo(nodePath, i)
		if err != nil {
			continue
		}
		devices = append(devices, dev)
	}

	return devices, nil
}

// getDeviceInfo 获取单个设备的详细信息
func (d *DCUDetector) getDeviceInfo(nodePath string, index int) (*detectors.Device, error) {
	dev := &detectors.Device{
		ID:          fmt.Sprintf("dcu-%d", index),
		HealthScore: 100.0,
	}

	// 读取设备名称
	if name, err := d.readNodeProperty(nodePath, "name"); err == nil {
		dev.Model = d.parseModelName(name)
	}

	// UUID - 使用节点路径生成
	dev.UUID = fmt.Sprintf("DCU-%04d-0000-0000-000000000000", index)

	// 显存信息
	if vram, err := d.readNodePropertyUint64(nodePath, "mem_banks/properties/size"); err == nil {
		dev.VRAMTotal = vram
		dev.VRAMFree = vram
		dev.VRAMUsed = 0
	}

	// PCIe Bus ID
	if busID, err := d.readNodeProperty(nodePath, "io_links/properties/node_id"); err == nil {
		dev.PCIEBusID = d.parsePCIEBusID(busID)
	} else {
		// 回退：从 sysfs 读取
		dev.PCIEBusID = d.getPCIEBusIDFromSysfs(index)
	}

	// 计算能力 - 根据型号估算
	dev.ComputeCap = d.estimateComputeCapability(dev.Model)

	// 温度 - 从 hwmon 读取
	if temp, err := d.readTemperature(index); err == nil {
		dev.Temperature = temp
		if temp > 85 {
			dev.HealthScore -= float64(temp-85) * 2
		}
	}

	// 功耗 - 从 power 读取
	if power, err := d.readPowerUsage(index); err == nil {
		dev.PowerUsage = power
	}

	// ECC 错误 - 海光 DCU 可能不支持 ECC，默认为 0
	dev.ECCErrors = 0

	// 确保健康分不低于 0
	if dev.HealthScore < 0 {
		dev.HealthScore = 0
	}

	return dev, nil
}

// readNodeProperty 读取节点属性
func (d *DCUDetector) readNodeProperty(nodePath, propPath string) (string, error) {
	fullPath := filepath.Join(nodePath, propPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// readNodePropertyUint64 读取节点属性（uint64）
func (d *DCUDetector) readNodePropertyUint64(nodePath, propPath string) (uint64, error) {
	str, err := d.readNodeProperty(nodePath, propPath)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(str, 10, 64)
}

// parseModelName 解析型号名称
func (d *DCUDetector) parseModelName(name string) string {
	// KFD 返回的名称可能是设备 ID，需要映射
	modelMap := map[string]string{
		"0x1006": "Hygon DCU Z100",
		"0x1007": "Hygon DCU Z100L",
	}

	if model, ok := modelMap[strings.ToLower(name)]; ok {
		return model
	}

	// 如果是纯数字，可能是节点 ID
	if _, err := strconv.ParseUint(name, 10, 64); err == nil {
		return "Hygon DCU Z100" // 默认型号
	}

	// 如果名称看起来像型号，直接返回
	if strings.Contains(strings.ToUpper(name), "DCU") || strings.Contains(strings.ToUpper(name), "HYGON") {
		return name
	}

	return "Hygon DCU Z100" // 默认型号
}

// parsePCIEBusID 解析 PCIe Bus ID
func (d *DCUDetector) parsePCIEBusID(nodeID string) string {
	// KFD node_id 格式可能是 "x:y:z"
	parts := strings.Split(nodeID, ":")
	if len(parts) >= 3 {
		bus, _ := strconv.ParseUint(parts[1], 10, 16)
		_, _ = strconv.ParseUint(parts[2], 10, 16) // device 号，保留用于未来使用
		return fmt.Sprintf("0000:%02x:00.0", bus)
	}
	return "0000:00:00.0"
}

// getPCIEBusIDFromSysfs 从 sysfs 获取 PCIe Bus ID
func (d *DCUDetector) getPCIEBusIDFromSysfs(index int) string {
	// 尝试从 drm 读取
	drmPath := fmt.Sprintf("/sys/class/drm/card%d/device", index)
	if link, err := os.Readlink(drmPath); err == nil {
		parts := strings.Split(link, "/")
		for i, part := range parts {
			if strings.HasPrefix(part, "0000:") {
				if i+1 < len(parts) {
					return filepath.Join(parts[i], parts[i+1])
				}
				return part
			}
		}
	}

	return fmt.Sprintf("0000:%02x:00.0", index)
}

// estimateComputeCapability 估算计算能力
func (d *DCUDetector) estimateComputeCapability(model string) detectors.ComputeCapability {
	// 海光 DCU 型号及算力 (基于公开规格)
	knownModels := map[string][2]uint64{
		"Hygon DCU Z100":  {95, 47},  // FP16: ~95 TFLOPS, FP32: ~47 TFLOPS
		"Hygon DCU Z100L": {120, 60}, // FP16: ~120 TFLOPS, FP32: ~60 TFLOPS
	}

	if tflops, ok := knownModels[model]; ok {
		return detectors.ComputeCapability{
			FP16TFLOPS: tflops[0],
			FP32TFLOPS: tflops[1],
			Major:      9, // 海光 DCU 兼容 CDNA 架构
			Minor:      0,
		}
	}

	// 默认值
	return detectors.ComputeCapability{
		FP16TFLOPS: 95,
		FP32TFLOPS: 47,
		Major:      9,
		Minor:      0,
	}
}

// readTemperature 读取温度
func (d *DCUDetector) readTemperature(index int) (uint32, error) {
	// 尝试从 hwmon 读取
	hwmonPath := fmt.Sprintf("/sys/class/drm/card%d/device/hwmon/hwmon*/temp1_input", index)
	if matches, err := filepath.Glob(hwmonPath); err == nil && len(matches) > 0 {
		if data, err := os.ReadFile(matches[0]); err == nil {
			temp, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 32)
			return uint32(temp / 1000), nil // 毫摄氏度转摄氏度
		}
	}

	// 返回默认值
	return 35, nil
}

// readPowerUsage 读取功耗
func (d *DCUDetector) readPowerUsage(index int) (uint32, error) {
	// 尝试从 amdgpu 读取功耗
	powerPath := fmt.Sprintf("/sys/class/hwmon/hwmon%d/power1_average", index)
	if data, err := os.ReadFile(powerPath); err == nil {
		power, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 32)
		return uint32(power / 1000000), nil // 微瓦转瓦
	}

	// 返回默认值
	return 200, nil
}

// GetTopology 获取设备间拓扑关系
func (d *DCUDetector) GetTopology(ctx context.Context) (*detectors.Topology, error) {
	if err := d.init(); err != nil {
		return nil, err
	}

	topology := &detectors.Topology{
		Devices: make([]detectors.TopologyDevice, 0, d.deviceCount),
		Links:   make([]detectors.TopologyLink, 0),
	}

	// 收集设备信息
	for i := 0; i < d.deviceCount; i++ {
		_ = fmt.Sprintf("/sys/class/kfd/kfd/topology/nodes/node%d", i) // nodePath 保留用于未来使用
		id := fmt.Sprintf("dcu-%d", i)
		pcieBusID := d.getPCIEBusIDFromSysfs(i)

		topology.Devices = append(topology.Devices, detectors.TopologyDevice{
			ID:        id,
			PCIEBusID: pcieBusID,
		})
	}

	// 检测设备间连接
	for i := 0; i < d.deviceCount; i++ {
		for j := i + 1; j < d.deviceCount; j++ {
			linkType, bandwidth := d.detectLink(i, j)
			if linkType != detectors.LinkTypeUnknown {
				topology.Links = append(topology.Links, detectors.TopologyLink{
					SourceID:  fmt.Sprintf("dcu-%d", i),
					TargetID:  fmt.Sprintf("dcu-%d", j),
					Type:      linkType,
					Bandwidth: bandwidth,
				})
			}
		}
	}

	return topology, nil
}

// detectLink 检测两个设备间的连接类型
func (d *DCUDetector) detectLink(dev1, dev2 int) (detectors.LinkType, uint64) {
	// 检查 xGMI 连接（海光高速互连技术）
	nodePath1 := fmt.Sprintf("/sys/class/kfd/kfd/topology/nodes/node%d", dev1)
	_ = fmt.Sprintf("/sys/class/kfd/kfd/topology/nodes/node%d", dev2) // 保留用于可能的未来使用

	// 读取 io_links 检查连接类型
	linksPath1 := filepath.Join(nodePath1, "io_links")
	if entries, err := os.ReadDir(linksPath1); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				linkTypePath := filepath.Join(linksPath1, entry.Name(), "properties/type")
				if linkType, err := os.ReadFile(linkTypePath); err == nil {
					typeStr := strings.TrimSpace(string(linkType))
					if typeStr == "xGMI" {
						// xGMI 连接，带宽约 100 GB/s
						return "xGMI", 100
					}
				}
			}
		}
	}

	// 检查 PCIe 连接
	// 海光 DCU 之间至少有 PCIe 连接
	return detectors.LinkTypePCIe, 32 // PCIe 4.0 x16 约 32 GB/s
}

// Close 关闭检测器
func (d *DCUDetector) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.initialized = false
	return nil
}

// Ensure DCUDetector implements Detector interface
var _ detectors.Detector = (*DCUDetector)(nil)
