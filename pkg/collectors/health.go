package collectors

import (
	"context"
	"fmt"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// HealthCollector 健康状态采集器
type HealthCollector struct{}

// NewHealthCollector 创建健康采集器
func NewHealthCollector() Collector {
	return &HealthCollector{}
}

// Collect 采集健康状态
func (c *HealthCollector) Collect(ctx context.Context, device *detectors.Device) (*Metrics, error) {
	// TODO: Phase 1 实现 - 采集设备健康状态
	// TODO: Phase 3 使用 eBPF 进行微秒级监控
	return &Metrics{
		Health: &HealthMetrics{
			Score: 100.0,
		},
	}, fmt.Errorf("not implemented yet")
}
