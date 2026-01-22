package detectors

import (
	"context"
)

// Detector 硬件检测器接口
type Detector interface {
	// Name 返回检测器名称
	Name() string

	// Detect 检测硬件类型和可用性
	Detect(ctx context.Context) (*HardwareType, error)

	// GetDevices 获取设备列表
	GetDevices(ctx context.Context) ([]*Device, error)

	// GetTopology 获取设备间拓扑关系
	GetTopology(ctx context.Context) (*Topology, error)

	// Close 关闭检测器，释放资源
	Close() error
}

// HardwareType 硬件类型信息
type HardwareType struct {
	Vendor          string // nvidia, huawei, hygon, cambricon
	DriverVersion   string // 驱动版本
	DriverAvailable bool   // 驱动是否可用
}

// Device 设备信息
type Device struct {
	ID          string
	UUID        string
	Model       string
	VRAMTotal   uint64 // bytes
	VRAMFree    uint64 // bytes
	VRAMUsed    uint64 // bytes
	PCIEBusID   string
	ComputeCap  ComputeCapability
	Temperature uint32  // Celsius
	PowerUsage  uint32  // Watts
	HealthScore float64 // 0-100
	ECCErrors   uint64  // ECC error count
}

// ComputeCapability 计算能力
type ComputeCapability struct {
	FP16TFLOPS uint64
	FP32TFLOPS uint64
	Major      int // CUDA compute capability major
	Minor      int // CUDA compute capability minor
}

// Topology 设备拓扑信息
type Topology struct {
	Devices []TopologyDevice
	Links   []TopologyLink
}

// TopologyDevice 拓扑中的设备
type TopologyDevice struct {
	ID        string
	PCIEBusID string
}

// TopologyLink 设备间连接
type TopologyLink struct {
	SourceID  string
	TargetID  string
	Type      LinkType // NVLink, PCIe, etc.
	Bandwidth uint64   // GB/s
}

// LinkType 连接类型
type LinkType string

const (
	LinkTypeNVLink  LinkType = "NVLink"
	LinkTypeHCCS    LinkType = "HCCS"
	LinkTypePCIe    LinkType = "PCIe"
	LinkTypeUnknown LinkType = "Unknown"
)
