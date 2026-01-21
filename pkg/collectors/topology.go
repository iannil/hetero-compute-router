package collectors

import (
	"context"
	"fmt"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// TopologyCollector 拓扑信息采集器
type TopologyCollector struct{}

// NewTopologyCollector 创建拓扑采集器
func NewTopologyCollector() Collector {
	return &TopologyCollector{}
}

// Collect 采集拓扑信息
func (c *TopologyCollector) Collect(ctx context.Context, device *detectors.Device) (*Metrics, error) {
	// TODO: Phase 1 实现 - 采集设备拓扑信息
	// TODO: 分析 PCIe/NVLink/HCCS 互联拓扑
	return &Metrics{
		Topology: &TopologyMetrics{
			Peers: []PeerInfo{},
		},
	}, fmt.Errorf("not implemented yet")
}
