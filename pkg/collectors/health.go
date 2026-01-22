package collectors

import (
	"context"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// HealthCollector 健康状态采集器
type HealthCollector struct {
	// 健康阈值配置
	TempWarningThreshold  uint32  // 温度警告阈值（摄氏度）
	TempCriticalThreshold uint32  // 温度临界阈值（摄氏度）
	ECCErrorThreshold     uint64  // ECC 错误阈值
	MinHealthScore        float64 // 最低健康分数
}

// NewHealthCollector 创建健康采集器
func NewHealthCollector() *HealthCollector {
	return &HealthCollector{
		TempWarningThreshold:  80,
		TempCriticalThreshold: 90,
		ECCErrorThreshold:     10,
		MinHealthScore:        0,
	}
}

// Name 返回采集器名称
func (c *HealthCollector) Name() string {
	return "health"
}

// Collect 采集健康状态
func (c *HealthCollector) Collect(ctx context.Context, devices []*detectors.Device, topology *detectors.Topology) (*Metrics, error) {
	if len(devices) == 0 {
		return &Metrics{
			Health: &HealthMetrics{Score: 100.0},
		}, nil
	}

	metrics := &Metrics{
		Health:        &HealthMetrics{},
		DeviceMetrics: make([]DeviceMetric, 0, len(devices)),
	}

	// 聚合所有设备的健康信息
	var totalScore float64
	var totalTemp, totalPower uint32
	var totalECCErrors uint64
	var deviceCount int

	for _, dev := range devices {
		// 计算单个设备的健康分数
		score := c.calculateHealthScore(dev)

		devMetric := DeviceMetric{
			DeviceID: dev.ID,
			Health: &HealthMetrics{
				Score:       score,
				Temperature: dev.Temperature,
				PowerUsage:  dev.PowerUsage,
				ECCErrors:   dev.ECCErrors,
			},
		}
		metrics.DeviceMetrics = append(metrics.DeviceMetrics, devMetric)

		totalScore += score
		totalTemp += dev.Temperature
		totalPower += dev.PowerUsage
		totalECCErrors += dev.ECCErrors
		deviceCount++
	}

	// 计算平均值
	if deviceCount > 0 {
		metrics.Health.Score = totalScore / float64(deviceCount)
		metrics.Health.Temperature = totalTemp / uint32(deviceCount)
		metrics.Health.PowerUsage = totalPower
		metrics.Health.ECCErrors = totalECCErrors
	}

	return metrics, nil
}

// calculateHealthScore 计算单个设备的健康分数
func (c *HealthCollector) calculateHealthScore(dev *detectors.Device) float64 {
	score := 100.0

	// 温度扣分
	if dev.Temperature > c.TempCriticalThreshold {
		// 临界温度，大幅扣分
		score -= float64(dev.Temperature-c.TempCriticalThreshold) * 5
	} else if dev.Temperature > c.TempWarningThreshold {
		// 警告温度，适度扣分
		score -= float64(dev.Temperature-c.TempWarningThreshold) * 2
	}

	// ECC 错误扣分
	if dev.ECCErrors > 0 {
		if dev.ECCErrors > c.ECCErrorThreshold {
			score -= 30 // 大量 ECC 错误，严重扣分
		} else {
			score -= float64(dev.ECCErrors) * 3
		}
	}

	// 使用检测器提供的健康分数作为参考
	if dev.HealthScore < 100 {
		// 综合考虑检测器的健康分数
		score = (score + dev.HealthScore) / 2
	}

	// 确保分数在有效范围内
	if score < c.MinHealthScore {
		score = c.MinHealthScore
	}
	if score > 100 {
		score = 100
	}

	return score
}

// IsHealthy 判断设备是否健康
func (c *HealthCollector) IsHealthy(score float64) bool {
	return score >= 60.0
}

// IsWarning 判断设备是否处于警告状态
func (c *HealthCollector) IsWarning(score float64) bool {
	return score >= 30.0 && score < 60.0
}

// IsCritical 判断设备是否处于临界状态
func (c *HealthCollector) IsCritical(score float64) bool {
	return score < 30.0
}

// Ensure HealthCollector implements Collector interface
var _ Collector = (*HealthCollector)(nil)
