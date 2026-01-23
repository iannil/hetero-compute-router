package ebpf

import (
	"testing"
	"time"
)

func TestHealthScoreThresholds(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		expected string
	}{
		{"Excellent score", 95, "excellent"},
		{"Good score", 80, "good"},
		{"Fair score", 65, "fair"},
		{"Poor score", 50, "poor"},
		{"Critical score", 30, "critical"},
		{"Failure score", 10, "failure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := GetHealthLevel(tt.score)
			if level != tt.expected {
				t.Errorf("GetHealthLevel(%v) = %v, want %v", tt.score, level, tt.expected)
			}
		})
	}
}

func TestDefaultThresholds(t *testing.T) {
	thresholds := DefaultThresholds()

	if thresholds.TemperatureAnomaly != 3.0 {
		t.Errorf("Expected TemperatureAnomaly 3.0, got %v", thresholds.TemperatureAnomaly)
	}

	if thresholds.TemperatureCritical != 90 {
		t.Errorf("Expected TemperatureCritical 90, got %v", thresholds.TemperatureCritical)
	}

	if thresholds.ClockDegradation != 100 {
		t.Errorf("Expected ClockDegradation 100, got %v", thresholds.ClockDegradation)
	}
}

func TestNewHealthAnalyzer(t *testing.T) {
	analyzer := NewHealthAnalyzer()

	if analyzer == nil {
		t.Fatal("NewHealthAnalyzer() returned nil")
	}

	if len(analyzer.GetAllDeviceIDs()) != 0 {
		t.Errorf("Expected no device IDs, got %v", analyzer.GetAllDeviceIDs())
	}
}

func TestEventBuffer(t *testing.T) {
	buf := NewEventBuffer(3)

	// 测试添加事件
	buf.Add("event1")
	buf.Add("event2")

	if buf.Count() != 2 {
		t.Errorf("Expected count 2, got %d", buf.Count())
	}

	// 测试获取事件
	events := buf.GetAll()
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}

	// 测试覆盖
	buf.Add("event3")
	buf.Add("event4")

	events = buf.GetAll()
	if len(events) != 3 {
		t.Errorf("Expected 3 events (buffer full), got %d", len(events))
	}

	// 测试清空
	buf.Clear()
	if buf.Count() != 0 {
		t.Errorf("Expected count 0 after clear, got %d", buf.Count())
	}
}

func TestHealthAnalyzer_AddGPUEvent(t *testing.T) {
	analyzer := NewHealthAnalyzer()

	event := GPUEvent{
		DeviceID:    0,
		Timestamp:   time.Now(),
		CoreClock:   1500,
		MemoryClock: 700,
		Power:       250000, // 250W in mW
		Temperature: 45,
		Utilization: 80,
	}

	analyzer.AddGPUEvent(event)

	ids := analyzer.GetAllDeviceIDs()
	if len(ids) != 1 {
		t.Errorf("Expected 1 device ID, got %d", len(ids))
	}

	if ids[0] != 0 {
		t.Errorf("Expected device ID 0, got %d", ids[0])
	}
}

func TestHealthAnalyzer_GetSnapshot(t *testing.T) {
	analyzer := NewHealthAnalyzer()

	// 添加一些事件
	event := GPUEvent{
		DeviceID:    0,
		Timestamp:   time.Now(),
		CoreClock:   1500,
		MemoryClock: 700,
		Power:       250000,
		Temperature: 45,
		Utilization: 80,
	}

	analyzer.AddGPUEvent(event)
	analyzer.AddGPUEvent(GPUEvent{
		DeviceID:    0,
		Timestamp:   time.Now().Add(time.Second),
		CoreClock:   1400,
		MemoryClock: 700,
		Power:       240000,
		Temperature: 50,
		Utilization: 85,
	})

	snapshot := analyzer.GetSnapshot(0)

	if snapshot == nil {
		t.Fatal("GetSnapshot() returned nil")
	}

	if snapshot.DeviceID != 0 {
		t.Errorf("Expected DeviceID 0, got %d", snapshot.DeviceID)
	}

	if snapshot.CoreClock != 1400 {
		t.Errorf("Expected CoreClock 1400 (latest), got %d", snapshot.CoreClock)
	}

	if snapshot.Temperature != 50 {
		t.Errorf("Expected Temperature 50 (latest), got %d", snapshot.Temperature)
	}
}

func TestHealthAnalyzer_ComputeHealthScore(t *testing.T) {
	analyzer := NewHealthAnalyzer()

	tests := []struct {
		name     string
		snapshot DeviceHealthSnapshot
		minScore float64
		maxScore float64
	}{
		{
			name: "Healthy device",
			snapshot: DeviceHealthSnapshot{
				Temperature:         45,
				IsThrottling:        false,
				ECCSingleBitCount:   0,
				ECCDoubleBitCount:   0,
				TemperatureTrend:    0,
				ECCErrorRate:        0,
			},
			minScore: 90,
			maxScore: 100,
		},
		{
			name: "High temperature",
			snapshot: DeviceHealthSnapshot{
				Temperature:         88,
				IsThrottling:        false,
				ECCSingleBitCount:   0,
				ECCDoubleBitCount:   0,
				TemperatureTrend:    0,
				ECCErrorRate:        0,
			},
			minScore: 88,
			maxScore: 94,
		},
		{
			name: "Critical temperature",
			snapshot: DeviceHealthSnapshot{
				Temperature:         95,
				IsThrottling:        false,
				ECCSingleBitCount:   0,
				ECCDoubleBitCount:   0,
				TemperatureTrend:    0,
				ECCErrorRate:        0,
			},
			minScore: 0,
			maxScore: 80,
		},
		{
			name: "Throttling",
			snapshot: DeviceHealthSnapshot{
				Temperature:         85,
				IsThrottling:        true,
				ThrottleDuration:    time.Minute,
				ECCSingleBitCount:   0,
				ECCDoubleBitCount:   0,
				TemperatureTrend:    0,
				ECCErrorRate:        0,
			},
			minScore: 70,
			maxScore: 90,
		},
		{
			name: "ECC errors",
			snapshot: DeviceHealthSnapshot{
				Temperature:         50,
				IsThrottling:        false,
				ECCSingleBitCount:   10,
				ECCDoubleBitCount:   0,
				TemperatureTrend:    0,
				ECCErrorRate:        0,
			},
			minScore: 85,
			maxScore: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := analyzer.ComputeHealthScore(&tt.snapshot)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("ComputeHealthScore() = %v, want between %v and %v", score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestHealthAnalyzer_ResetDevice(t *testing.T) {
	analyzer := NewHealthAnalyzer()

	// 添加事件
	event := GPUEvent{
		DeviceID:  0,
		Timestamp: time.Now(),
	}
	analyzer.AddGPUEvent(event)

	// 重置
	analyzer.ResetDevice(0)

	// 验证已清除
	ids := analyzer.GetAllDeviceIDs()
	if len(ids) != 0 {
		t.Errorf("Expected 0 device IDs after reset, got %d", len(ids))
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if !config.Enabled {
		t.Error("Expected Enabled to be true by default")
	}

	if config.GPUSampleInterval != 100*time.Millisecond {
		t.Errorf("Expected GPUSampleInterval 100ms, got %v", config.GPUSampleInterval)
	}

	if config.BufferSize != 1000 {
		t.Errorf("Expected BufferSize 1000, got %d", config.BufferSize)
	}
}
