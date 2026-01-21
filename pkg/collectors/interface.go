package collectors

import (
	"context"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// Collector 指标采集器接口
type Collector interface {
	// Collect 采集指标
	Collect(ctx context.Context, device *detectors.Device) (*Metrics, error)
}

// Metrics 设备指标
type Metrics struct {
	// 指纹信息
	Fingerprint *FingerprintMetrics

	// 健康状态
	Health *HealthMetrics

	// 拓扑信息
	Topology *TopologyMetrics
}

// FingerprintMetrics 算力指纹指标
type FingerprintMetrics struct {
	VRAMTotal      uint64 // bytes
	VRAMUsed       uint64 // bytes
	ComputeCap     ComputeMetrics
	Interconnect   string // NVLink, HCCS, PCIe
	InterconnectBw uint64 // MB/s
}

// ComputeMetrics 计算性能指标
type ComputeMetrics struct {
	FP16TFLOPS uint64
	FP32TFLOPS uint64
}

// HealthMetrics 健康状态指标
type HealthMetrics struct {
	Score          float64 // 0-100, 越高越健康
	Temperature    uint32  // 摄氏度
	PowerUsage     uint32  // 瓦特
	PowerLimit     uint32  // 瓦特
	ECCErrors      uint64
	UtilizationGPU uint32 // 百分比
	UtilizationMEM uint32 // 百分比
}

// TopologyMetrics 拓扑指标
type TopologyMetrics struct {
	Peers        []PeerInfo
	NumaNode     int32
	PCIEDomain   uint32
	PCIEBus      uint32
	PCIEDevice   uint32
	PCIEFunction uint32
}

// PeerInfo 对等设备信息（互联拓扑）
type PeerInfo struct {
	DeviceID string
	LinkType string // NVLink, HCCS, PCIe, QPI
	LinkBw   uint64 // MB/s
	Hops     int    // 跳数
}
