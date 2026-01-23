package ebpf

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/zrs-products/hetero-compute-router/pkg/collectors"
	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// EBPFHealthCollector 基于 eBPF 的健康指标采集器
// 实现 collectors.Collector 接口，集成到采集器框架
type EBPFHealthCollector struct {
	manager *EBPFManager
	mu      sync.RWMutex
	once    sync.Once
}

// NewEBPFHealthCollector 创建 eBPF 健康采集器
func NewEBPFHealthCollector(config *Config) (*EBPFHealthCollector, error) {
	if config == nil {
		config = DefaultConfig()
	}

	manager, err := NewEBPFManager(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create eBPF manager: %w", err)
	}

	c := &EBPFHealthCollector{
		manager: manager,
	}

	// 启动监控
	if err := c.manager.Start(); err != nil {
		return nil, fmt.Errorf("failed to start eBPF manager: %w", err)
	}

	return c, nil
}

// Name 返回采集器名称
func (c *EBPFHealthCollector) Name() string {
	return "ebpf-health"
}

// Collect 采集指标
func (c *EBPFHealthCollector) Collect(ctx context.Context, devices []*detectors.Device, topology *detectors.Topology) (*collectors.Metrics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 添加设备到监控
	c.manager.AddDevices(devices)

	// 获取所有设备的快照
	metrics := &collectors.Metrics{
		Health:        &collectors.HealthMetrics{},
		DeviceMetrics: make([]collectors.DeviceMetric, 0, len(devices)),
	}

	totalScore := 0.0
	deviceCount := 0

	// 为每个设备创建指标
	for _, dev := range devices {
		deviceID := c.parseDeviceID(dev.ID)
		if deviceID == 0 {
			continue
		}

		snapshot := c.manager.GetSnapshot(deviceID)
		if snapshot == nil {
			// 如果没有快照，使用设备基础信息
			snapshot = &DeviceHealthSnapshot{
				DeviceID:    deviceID,
				Temperature: dev.Temperature,
				Power:       dev.PowerUsage,
				HealthScore: dev.HealthScore,
			}
		}

		dm := collectors.DeviceMetric{
			DeviceID: dev.ID,
			Health: &collectors.HealthMetrics{
				Score:          snapshot.HealthScore,
				Temperature:    snapshot.Temperature,
				PowerUsage:     snapshot.Power,
				ECCErrors:      snapshot.ECCSingleBitCount + snapshot.ECCDoubleBitCount,
				UtilizationGPU: snapshot.Utilization,
			},
		}

		metrics.DeviceMetrics = append(metrics.DeviceMetrics, dm)

		totalScore += snapshot.HealthScore
		deviceCount++

		// 检查预测性故障
		if isFailure, _ := c.manager.IsPredictiveFailure(deviceID); isFailure {
			// 可以在这里触发告警
			metrics.Health.ECCErrors++
		}
	}

	// 计算聚合健康分
	if deviceCount > 0 {
		metrics.Health.Score = totalScore / float64(deviceCount)
	}

	return metrics, nil
}

// parseDeviceID 从设备 ID 字符串解析数字 ID
func (c *EBPFHealthCollector) parseDeviceID(id string) uint32 {
	// 支持 "gpu-0", "dcu-0", "npu-0" 等格式
	var num int
	if n, err := fmt.Sscanf(id, "gpu-%d", &num); n == 1 && err == nil {
		return uint32(num)
	}
	if n, err := fmt.Sscanf(id, "dcu-%d", &num); n == 1 && err == nil {
		return uint32(num)
	}
	if n, err := fmt.Sscanf(id, "npu-%d", &num); n == 1 && err == nil {
		return uint32(num)
	}

	// 尝试从最后提取数字
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '-' {
			if num, err := strconv.Atoi(id[i+1:]); err == nil {
				return uint32(num)
			}
		}
	}

	return 0
}

// Close 关闭采集器
func (c *EBPFHealthCollector) Close() error {
	return c.manager.Stop()
}

// IsEnabled 返回是否启用了 eBPF
func (c *EBPFHealthCollector) IsEnabled() bool {
	return c.manager.IsEnabled()
}

// GetManager 获取 eBPF 管理器（用于高级操作）
func (c *EBPFHealthCollector) GetManager() *EBPFManager {
	return c.manager
}

// Ensure EBPFHealthCollector implements Collector interface
var _ collectors.Collector = (*EBPFHealthCollector)(nil)
