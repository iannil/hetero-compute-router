package collectors

import (
	"context"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// FingerprintCollector 算力指纹采集器
type FingerprintCollector struct{}

// NewFingerprintCollector 创建指纹采集器
func NewFingerprintCollector() *FingerprintCollector {
	return &FingerprintCollector{}
}

// Name 返回采集器名称
func (c *FingerprintCollector) Name() string {
	return "fingerprint"
}

// Collect 采集算力指纹
func (c *FingerprintCollector) Collect(ctx context.Context, devices []*detectors.Device, topology *detectors.Topology) (*Metrics, error) {
	if len(devices) == 0 {
		return &Metrics{
			Fingerprint: &FingerprintMetrics{},
		}, nil
	}

	metrics := &Metrics{
		Fingerprint:   &FingerprintMetrics{},
		DeviceMetrics: make([]DeviceMetric, 0, len(devices)),
	}

	// 聚合所有设备的指纹信息
	var totalVRAM, totalVRAMUsed, totalVRAMFree uint64
	var totalFP16, totalFP32 uint64

	for _, dev := range devices {
		totalVRAM += dev.VRAMTotal
		totalVRAMUsed += dev.VRAMUsed
		totalVRAMFree += dev.VRAMFree
		totalFP16 += dev.ComputeCap.FP16TFLOPS
		totalFP32 += dev.ComputeCap.FP32TFLOPS

		// 每个设备的指纹
		devMetric := DeviceMetric{
			DeviceID: dev.ID,
			Fingerprint: &FingerprintMetrics{
				VRAMTotal: dev.VRAMTotal,
				VRAMUsed:  dev.VRAMUsed,
				VRAMFree:  dev.VRAMFree,
				ComputeCap: ComputeMetrics{
					FP16TFLOPS: dev.ComputeCap.FP16TFLOPS,
					FP32TFLOPS: dev.ComputeCap.FP32TFLOPS,
				},
			},
		}
		metrics.DeviceMetrics = append(metrics.DeviceMetrics, devMetric)
	}

	metrics.Fingerprint.VRAMTotal = totalVRAM
	metrics.Fingerprint.VRAMUsed = totalVRAMUsed
	metrics.Fingerprint.VRAMFree = totalVRAMFree
	metrics.Fingerprint.ComputeCap = ComputeMetrics{
		FP16TFLOPS: totalFP16,
		FP32TFLOPS: totalFP32,
	}

	// 从拓扑信息确定主要互联类型
	if topology != nil && len(topology.Links) > 0 {
		linkTypeCounts := make(map[detectors.LinkType]int)
		var maxBandwidth uint64
		for _, link := range topology.Links {
			linkTypeCounts[link.Type]++
			if link.Bandwidth > maxBandwidth {
				maxBandwidth = link.Bandwidth
			}
		}

		// 找出最常见的连接类型
		var dominantType detectors.LinkType
		var maxCount int
		for lt, count := range linkTypeCounts {
			if count > maxCount {
				maxCount = count
				dominantType = lt
			}
		}
		metrics.Fingerprint.Interconnect = string(dominantType)
		metrics.Fingerprint.InterconnectBw = maxBandwidth
	} else {
		metrics.Fingerprint.Interconnect = string(detectors.LinkTypePCIe)
		metrics.Fingerprint.InterconnectBw = 32 // PCIe 4.0 x16 default
	}

	return metrics, nil
}

// Ensure FingerprintCollector implements Collector interface
var _ Collector = (*FingerprintCollector)(nil)
