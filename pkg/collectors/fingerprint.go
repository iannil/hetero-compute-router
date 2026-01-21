package collectors

import (
	"context"
	"fmt"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// FingerprintCollector 算力指纹采集器
type FingerprintCollector struct{}

// NewFingerprintCollector 创建指纹采集器
func NewFingerprintCollector() Collector {
	return &FingerprintCollector{}
}

// Collect 采集算力指纹
func (c *FingerprintCollector) Collect(ctx context.Context, device *detectors.Device) (*Metrics, error) {
	// TODO: Phase 1 实现 - 采集设备指纹信息
	return &Metrics{
		Fingerprint: &FingerprintMetrics{
			VRAMTotal:    device.VRAMTotal,
			VRAMUsed:     device.VRAMTotal - device.VRAMFree,
			ComputeCap:   ComputeMetrics{},
			Interconnect: "",
		},
	}, fmt.Errorf("not implemented yet")
}
