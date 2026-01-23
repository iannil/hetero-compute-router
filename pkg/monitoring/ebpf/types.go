package ebpf

import "time"

// HealthEventType 健康事件类型
type HealthEventType string

const (
	EventECCSingleBit   HealthEventType = "ecc_sb"
	EventECCDoubleBit   HealthEventType = "ecc_db"
	EventPageRetire     HealthEventType = "page_retire"
	EventGPUReset       HealthEventType = "gpu_reset"
	EventClockThrottle  HealthEventType = "clock_throttle"
	EventPowerThrottle  HealthEventType = "power_throttle"
	EventThermalThrottle HealthEventType = "thermal_throttle"
)

// GPUEvent GPU 指标事件（来自 eBPF）
type GPUEvent struct {
	DeviceID        uint32
	Timestamp       time.Time
	CoreClock       uint32 // MHz
	MemoryClock     uint32 // MHz
	Power           uint32 // mW
	Temperature     uint32 // Celsius
	Utilization     uint32 // Percentage
	ThrottlingFlags uint8
}

// PCIeEvent PCIe 带宽事件（来自 eBPF）
type PCIeEvent struct {
	DeviceID    uint32
	Timestamp   time.Time
	ReadBytes   uint64
	WriteBytes  uint64
	ReplayCount uint32
}

// HealthEvent 健康相关事件（来自 eBPF）
type HealthEvent struct {
	DeviceID  uint32
	Timestamp time.Time
	Type      HealthEventType
	Count     uint32
	Address   uint64
}

// ThrottleReason 降频原因
type ThrottleReason string

const (
	ThrottleReasonNone   ThrottleReason = ""
	ThrottleReasonPower  ThrottleReason = "power"
	ThrottleReasonThermal ThrottleReason = "thermal"
	ThrottleReasonReliability ThrottleReason = "reliability"
)

// DeviceHealthSnapshot 设备健康快照
type DeviceHealthSnapshot struct {
	DeviceID uint32
	// 基础信息
	Timestamp time.Time
	// 当前指标
	CoreClock     uint32
	MemoryClock   uint32
	Temperature   uint32
	Power         uint32
	Utilization   uint32
	PCIeBandwidth float64 // GB/s
	// 降频状态
	IsThrottling     bool
	ThrottleReason   ThrottleReason
	ThrottleDuration time.Duration
	// 错误计数（滑动窗口内）
	ECCSingleBitCount uint64
	ECCDoubleBitCount uint64
	PageRetireCount   uint64
	// 计算的健康指标
	HealthScore       float64 // 0-100
	Confidence        float64 // 0-1
	// 趋势指标
	TemperatureTrend float64 // °C/sec
	PowerTrend       float64 // W/sec
	ECCErrorRate     float64 // errors/sec
	ClockDegradation float64 // MHz drop from baseline
}

// BaselineMetrics 正常运行参数基线
type BaselineMetrics struct {
	AverageTemperature float64
	AveragePower      float64
	AverageClock      float64
	StdDevTemp        float64
	StdDevPower       float64
	LastUpdated       time.Time
	SampleCount       uint32
}

// DetectionThresholds 设备特定的检测阈值
type DetectionThresholds struct {
	TemperatureAnomaly  float64 // sigma threshold
	PowerAnomaly        float64
	ClockDegradation    float64 // MHz drop threshold
	ECCRisingTrend      float64 // errors/sec^2 acceleration
	TemperatureCritical uint32  // Celsius
	PowerCritical       uint32  // Watts
}

// DefaultThresholds 返回默认检测阈值
func DefaultThresholds() *DetectionThresholds {
	return &DetectionThresholds{
		TemperatureAnomaly:  3.0,  // 3-sigma
		PowerAnomaly:        3.0,
		ClockDegradation:    100,  // 100 MHz drop
		ECCRisingTrend:      0.1,  // 0.1 errors/sec^2
		TemperatureCritical: 90,   // 90°C
		PowerCritical:       300,  // 300W
	}
}

// AnalysisWindow 分析窗口大小
const AnalysisWindow = 60 // 60秒

// HealthScoreThresholds 健康分阈值
const (
	HealthScoreExcellent = 90.0 // 优秀
	HealthScoreGood      = 75.0 // 良好
	HealthScoreFair      = 60.0 // 一般
	HealthScorePoor      = 40.0 // 较差
	HealthScoreCritical  = 25.0 // 危险
)

// GetHealthLevel 返回健康等级描述
func GetHealthLevel(score float64) string {
	switch {
	case score >= HealthScoreExcellent:
		return "excellent"
	case score >= HealthScoreGood:
		return "good"
	case score >= HealthScoreFair:
		return "fair"
	case score >= HealthScorePoor:
		return "poor"
	case score >= HealthScoreCritical:
		return "critical"
	default:
		return "failure"
	}
}

// EventBuffer 事件环形缓冲区
type EventBuffer struct {
	buffer []interface{}
	size   int
	index  int
	count  int
}

// NewEventBuffer 创建新的事件缓冲区
func NewEventBuffer(size int) *EventBuffer {
	return &EventBuffer{
		buffer: make([]interface{}, size),
		size:   size,
		index:  0,
		count:  0,
	}
}

// Add 添加事件到缓冲区
func (b *EventBuffer) Add(event interface{}) {
	b.buffer[b.index] = event
	b.index = (b.index + 1) % b.size
	if b.count < b.size {
		b.count++
	}
}

// GetAll 获取缓冲区中所有有效事件
func (b *EventBuffer) GetAll() []interface{} {
	if b.count < b.size {
		return b.buffer[:b.count]
	}

	// 缓冲区已满，返回从 index 开始的所有元素
	result := make([]interface{}, b.count)
	copy(result, b.buffer[b.index:])
	copy(result[b.size-b.index:], b.buffer[:b.index])
	return result
}

// Count 返回有效事件数量
func (b *EventBuffer) Count() int {
	return b.count
}

// Clear 清空缓冲区
func (b *EventBuffer) Clear() {
	b.index = 0
	b.count = 0
	for i := range b.buffer {
		b.buffer[i] = nil
	}
}
