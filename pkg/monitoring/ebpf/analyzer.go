package ebpf

import (
	"math"
	"sync"
	"time"
)

// HealthAnalyzer 健康分析器，进行趋势分析和预测性健康评分
type HealthAnalyzer struct {
	mu sync.RWMutex

	// 每设备的事件缓冲区
	gpuBuffers    map[uint32]*EventBuffer
	pcieBuffers   map[uint32]*EventBuffer
	healthBuffers map[uint32]*EventBuffer

	// 学习的基线值
	baselines map[uint32]*BaselineMetrics

	// 检测阈值
	thresholds map[uint32]*DetectionThresholds

	// 默认阈值
	defaultThresholds *DetectionThresholds
}

// NewHealthAnalyzer 创建新的健康分析器
func NewHealthAnalyzer() *HealthAnalyzer {
	return &HealthAnalyzer{
		gpuBuffers:         make(map[uint32]*EventBuffer),
		pcieBuffers:        make(map[uint32]*EventBuffer),
		healthBuffers:      make(map[uint32]*EventBuffer),
		baselines:          make(map[uint32]*BaselineMetrics),
		thresholds:         make(map[uint32]*DetectionThresholds),
		defaultThresholds:  DefaultThresholds(),
	}
}

// AddGPUEvent 添加 GPU 事件
func (a *HealthAnalyzer) AddGPUEvent(event GPUEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.gpuBuffers[event.DeviceID]; !ok {
		a.gpuBuffers[event.DeviceID] = NewEventBuffer(AnalysisWindow)
		a.baselines[event.DeviceID] = &BaselineMetrics{}
		a.thresholds[event.DeviceID] = DefaultThresholds()
	}
	a.gpuBuffers[event.DeviceID].Add(event)
}

// AddPCIeEvent 添加 PCIe 事件
func (a *HealthAnalyzer) AddPCIeEvent(event PCIeEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.pcieBuffers[event.DeviceID]; !ok {
		a.pcieBuffers[event.DeviceID] = NewEventBuffer(AnalysisWindow)
	}
	a.pcieBuffers[event.DeviceID].Add(event)
}

// AddHealthEvent 添加健康事件
func (a *HealthAnalyzer) AddHealthEvent(event HealthEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.healthBuffers[event.DeviceID]; !ok {
		a.healthBuffers[event.DeviceID] = NewEventBuffer(AnalysisWindow)
	}
	a.healthBuffers[event.DeviceID].Add(event)
}

// GetSnapshot 获取设备的健康快照
func (a *HealthAnalyzer) GetSnapshot(deviceID uint32) *DeviceHealthSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()

	snapshot := &DeviceHealthSnapshot{
		DeviceID:  deviceID,
		Timestamp: time.Now(),
		HealthScore: 100.0,
		Confidence: 0.5,
	}

	// 获取最新的 GPU 事件
	if buf, ok := a.gpuBuffers[deviceID]; ok && buf.Count() > 0 {
		events := buf.GetAll()
		if len(events) > 0 {
			latest := events[len(events)-1].(GPUEvent)
			snapshot.CoreClock = latest.CoreClock
			snapshot.MemoryClock = latest.MemoryClock
			snapshot.Temperature = latest.Temperature
			snapshot.Power = latest.Power
			snapshot.Utilization = latest.Utilization

			// 检查降频
			snapshot.IsThrottling = latest.ThrottlingFlags != 0
			if latest.ThrottlingFlags&0x01 != 0 {
				snapshot.ThrottleReason = ThrottleReasonPower
			} else if latest.ThrottlingFlags&0x02 != 0 {
				snapshot.ThrottleReason = ThrottleReasonThermal
			}

			// 计算趋势
			snapshot.TemperatureTrend = a.computeTemperatureTrend(events)
			snapshot.PowerTrend = a.computePowerTrend(events)
		}
	}

	// 获取 PCIe 带宽
	if buf, ok := a.pcieBuffers[deviceID]; ok && buf.Count() > 0 {
		events := buf.GetAll()
		snapshot.PCIeBandwidth = a.computePCIeBandwidth(events)
	}

	// 统计健康事件
	if buf, ok := a.healthBuffers[deviceID]; ok && buf.Count() > 0 {
		events := buf.GetAll()
		a.countHealthEvents(events, snapshot)
		snapshot.ECCErrorRate = a.computeECCErrorRate(events)
	}

	// 计算健康分
	snapshot.HealthScore = a.ComputeHealthScore(snapshot)

	// 更新基线
	a.updateBaseline(deviceID, snapshot)

	return snapshot
}

// computeTemperatureTrend 计算温度趋势 (°C/sec)
func (a *HealthAnalyzer) computeTemperatureTrend(events []interface{}) float64 {
	if len(events) < 2 {
		return 0
	}

	// 简单线性回归
	n := float64(len(events))
	var sumX, sumY, sumXY, sumX2 float64

	for i, evt := range events {
		e := evt.(GPUEvent)
		x := float64(i)
		y := float64(e.Temperature)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	return slope // 每个采样周期的温度变化
}

// computePowerTrend 计算功耗趋势 (W/sec)
func (a *HealthAnalyzer) computePowerTrend(events []interface{}) float64 {
	if len(events) < 2 {
		return 0
	}

	n := float64(len(events))
	var sumX, sumY, sumXY, sumX2 float64

	for i, evt := range events {
		e := evt.(GPUEvent)
		x := float64(i)
		y := float64(e.Power)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	return slope
}

// computePCIeBandwidth 计算 PCIe 带宽 (GB/s)
func (a *HealthAnalyzer) computePCIeBandwidth(events []interface{}) float64 {
	if len(events) == 0 {
		return 0
	}

	var totalRead, totalWrite uint64
	for _, evt := range events {
		e := evt.(PCIeEvent)
		totalRead += e.ReadBytes
		totalWrite += e.WriteBytes
	}

	// 简化计算：假设事件间隔为 1 秒
	totalBytes := totalRead + totalWrite
	return float64(totalBytes) / (1024 * 1024 * 1024) // GB
}

// countHealthEvents 统计健康事件
func (a *HealthAnalyzer) countHealthEvents(events []interface{}, snapshot *DeviceHealthSnapshot) {
	for _, evt := range events {
		e := evt.(HealthEvent)
		switch e.Type {
		case EventECCSingleBit:
			snapshot.ECCSingleBitCount += uint64(e.Count)
		case EventECCDoubleBit:
			snapshot.ECCDoubleBitCount += uint64(e.Count)
		case EventPageRetire:
			snapshot.PageRetireCount += uint64(e.Count)
		}
	}
}

// computeECCErrorRate 计算 ECC 错误率 (errors/sec)
func (a *HealthAnalyzer) computeECCErrorRate(events []interface{}) float64 {
	if len(events) < 2 {
		return 0
	}

	// 计算时间跨度
	first := events[0].(HealthEvent).Timestamp
	last := events[len(events)-1].(HealthEvent).Timestamp
	duration := last.Sub(first).Seconds()
	if duration <= 0 {
		return 0
	}

	// 统计总错误数
	var count uint64
	for _, evt := range events {
		e := evt.(HealthEvent)
		if e.Type == EventECCSingleBit || e.Type == EventECCDoubleBit {
			count += uint64(e.Count)
		}
	}

	return float64(count) / duration
}

// ComputeHealthScore 计算综合健康分 (0-100)
func (a *HealthAnalyzer) ComputeHealthScore(snapshot *DeviceHealthSnapshot) float64 {
	score := 100.0
	threshold := a.getThreshold(snapshot.DeviceID)

	// 温度评分 (0-25分)
	if snapshot.Temperature > threshold.TemperatureCritical {
		score -= 25
	} else if snapshot.Temperature > 85 {
		score -= float64(snapshot.Temperature-85) * 2
	}

	// 降频惩罚 (0-20分)
	if snapshot.IsThrottling {
		throttlePenalty := float64(snapshot.ThrottleDuration.Seconds()) / 60.0
		score -= throttlePenalty * 20
	}

	// ECC 错误惩罚 (0-30分)
	eccPenalty := float64(snapshot.ECCDoubleBitCount) * 20
	eccPenalty += float64(snapshot.ECCSingleBitCount) * 0.1
	score -= eccPenalty

	// 趋势惩罚 (预测性，0-15分)
	if snapshot.TemperatureTrend > 0.5 { // 温度快速上升
		score -= 10
	}
	if snapshot.ECCErrorRate > 1.0 { // 每秒超过 1 个错误
		score -= 10
	}

	// 时钟降频惩罚 (0-10分)
	baseline := a.getBaseline(snapshot.DeviceID)
	if baseline != nil && baseline.AverageClock > 0 {
		degradation := baseline.AverageClock - float64(snapshot.CoreClock)
		if degradation > threshold.ClockDegradation {
			score -= 10
		}
	}

	return math.Max(0, math.Min(100, score))
}

// IsPredictiveFailure 预测是否即将发生故障
func (a *HealthAnalyzer) IsPredictiveFailure(deviceID uint32) (bool, string) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	snapshot := a.GetSnapshot(deviceID)
	threshold := a.getThreshold(deviceID)

	reasons := []string{}

	// 温度急剧上升
	if snapshot.TemperatureTrend > 1.0 {
		reasons = append(reasons, "temperature_rising_rapidly")
	}

	// ECC 错误加速
	if snapshot.ECCErrorRate > 0.5 && snapshot.ECCDoubleBitCount > 0 {
		reasons = append(reasons, "ecc_errors_accelerating")
	}

	// 频繁降频
	if snapshot.IsThrottling && snapshot.ThrottleDuration > 10*time.Minute {
		reasons = append(reasons, "sustained_throttling")
	}

	// 严重时钟降频
	baseline := a.getBaseline(deviceID)
	if baseline != nil && baseline.AverageClock > 0 {
		degradation := baseline.AverageClock - float64(snapshot.CoreClock)
		if degradation > threshold.ClockDegradation*2 {
			reasons = append(reasons, "severe_clock_degradation")
		}
	}

	// 健康分过低
	if snapshot.HealthScore < HealthScoreCritical {
		reasons = append(reasons, "health_score_critical")
	}

	return len(reasons) > 0, ""
}

// getThreshold 获取设备的检测阈值
func (a *HealthAnalyzer) getThreshold(deviceID uint32) *DetectionThresholds {
	if t, ok := a.thresholds[deviceID]; ok {
		return t
	}
	return a.defaultThresholds
}

// getBaseline 获取设备的基线
func (a *HealthAnalyzer) getBaseline(deviceID uint32) *BaselineMetrics {
	return a.baselines[deviceID]
}

// updateBaseline 更新设备的运行基线
func (a *HealthAnalyzer) updateBaseline(deviceID uint32, snapshot *DeviceHealthSnapshot) {
	baseline := a.baselines[deviceID]
	if baseline == nil {
		baseline = &BaselineMetrics{}
		a.baselines[deviceID] = baseline
	}

	// 指数移动平均
	alpha := 0.1
	baseline.AverageTemperature = alpha*float64(snapshot.Temperature) + (1-alpha)*baseline.AverageTemperature
	baseline.AveragePower = alpha*float64(snapshot.Power) + (1-alpha)*baseline.AveragePower
	baseline.AverageClock = alpha*float64(snapshot.CoreClock) + (1-alpha)*baseline.AverageClock

	baseline.SampleCount++
	baseline.LastUpdated = time.Now()
}

// SetThreshold 设置设备的检测阈值
func (a *HealthAnalyzer) SetThreshold(deviceID uint32, threshold *DetectionThresholds) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.thresholds[deviceID] = threshold
}

// GetBaseline 获取设备的基线（外部调用）
func (a *HealthAnalyzer) GetBaseline(deviceID uint32) (*BaselineMetrics, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	baseline, ok := a.baselines[deviceID]
	return baseline, ok
}

// ResetDevice 重置设备的分析状态
func (a *HealthAnalyzer) ResetDevice(deviceID uint32) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.gpuBuffers, deviceID)
	delete(a.pcieBuffers, deviceID)
	delete(a.healthBuffers, deviceID)
	delete(a.baselines, deviceID)
	delete(a.thresholds, deviceID)
}

// GetAllDeviceIDs 获取所有已分析的设备 ID
func (a *HealthAnalyzer) GetAllDeviceIDs() []uint32 {
	a.mu.RLock()
	defer a.mu.RUnlock()

	ids := make([]uint32, 0, len(a.gpuBuffers))
	for id := range a.gpuBuffers {
		ids = append(ids, id)
	}
	return ids
}
