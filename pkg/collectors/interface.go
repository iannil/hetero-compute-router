package collectors

import (
	"context"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// Collector 指标采集器接口
type Collector interface {
	// Name 返回采集器名称
	Name() string

	// Collect 采集指标
	Collect(ctx context.Context, devices []*detectors.Device, topology *detectors.Topology) (*Metrics, error)
}

// Metrics 设备指标（包含所有设备的聚合信息）
type Metrics struct {
	// 指纹信息
	Fingerprint *FingerprintMetrics

	// 健康状态
	Health *HealthMetrics

	// 拓扑信息
	Topology *TopologyMetrics

	// 每个设备的详细指标
	DeviceMetrics []DeviceMetric
}

// DeviceMetric 单个设备的指标
type DeviceMetric struct {
	DeviceID    string
	Fingerprint *FingerprintMetrics
	Health      *HealthMetrics
}

// FingerprintMetrics 算力指纹指标
type FingerprintMetrics struct {
	VRAMTotal      uint64 // bytes
	VRAMUsed       uint64 // bytes
	VRAMFree       uint64 // bytes
	ComputeCap     ComputeMetrics
	Interconnect   string // NVLink, HCCS, PCIe
	InterconnectBw uint64 // GB/s
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
	LinkBw   uint64 // GB/s
	Hops     int    // 跳数
}

// AggregatedMetrics 聚合后的节点级指标
type AggregatedMetrics struct {
	TotalVRAM        uint64
	TotalVRAMUsed    uint64
	TotalVRAMFree    uint64
	TotalFP16FLOPS   uint64
	TotalFP32FLOPS   uint64
	AvgHealthScore   float64
	DeviceCount      int
	InterconnectType string
}

// Aggregate 聚合多个设备的指标
func (m *Metrics) Aggregate() *AggregatedMetrics {
	agg := &AggregatedMetrics{}

	if m.Fingerprint != nil {
		agg.TotalVRAM = m.Fingerprint.VRAMTotal
		agg.TotalVRAMUsed = m.Fingerprint.VRAMUsed
		agg.TotalVRAMFree = m.Fingerprint.VRAMFree
		agg.TotalFP16FLOPS = m.Fingerprint.ComputeCap.FP16TFLOPS
		agg.TotalFP32FLOPS = m.Fingerprint.ComputeCap.FP32TFLOPS
		agg.InterconnectType = m.Fingerprint.Interconnect
	}

	if m.Health != nil {
		agg.AvgHealthScore = m.Health.Score
	}

	agg.DeviceCount = len(m.DeviceMetrics)

	return agg
}
