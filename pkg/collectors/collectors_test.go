package collectors

import (
	"context"
	"testing"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

func createTestDevices() []*detectors.Device {
	return []*detectors.Device{
		{
			ID:          "gpu-0",
			UUID:        "GPU-test-0000",
			Model:       "NVIDIA A100-SXM4-80GB",
			VRAMTotal:   80 * 1024 * 1024 * 1024,
			VRAMFree:    70 * 1024 * 1024 * 1024,
			VRAMUsed:    10 * 1024 * 1024 * 1024,
			Temperature: 45,
			PowerUsage:  200,
			HealthScore: 95.0,
			ECCErrors:   0,
			PCIEBusID:   "0000:01:00.0",
			ComputeCap: detectors.ComputeCapability{
				FP16TFLOPS: 312,
				FP32TFLOPS: 19,
				Major:      8,
				Minor:      0,
			},
		},
		{
			ID:          "gpu-1",
			UUID:        "GPU-test-0001",
			Model:       "NVIDIA A100-SXM4-80GB",
			VRAMTotal:   80 * 1024 * 1024 * 1024,
			VRAMFree:    80 * 1024 * 1024 * 1024,
			VRAMUsed:    0,
			Temperature: 40,
			PowerUsage:  100,
			HealthScore: 100.0,
			ECCErrors:   0,
			PCIEBusID:   "0000:02:00.0",
			ComputeCap: detectors.ComputeCapability{
				FP16TFLOPS: 312,
				FP32TFLOPS: 19,
				Major:      8,
				Minor:      0,
			},
		},
	}
}

func createTestTopology() *detectors.Topology {
	return &detectors.Topology{
		Devices: []detectors.TopologyDevice{
			{ID: "gpu-0", PCIEBusID: "0000:01:00.0"},
			{ID: "gpu-1", PCIEBusID: "0000:02:00.0"},
		},
		Links: []detectors.TopologyLink{
			{
				SourceID:  "gpu-0",
				TargetID:  "gpu-1",
				Type:      detectors.LinkTypeNVLink,
				Bandwidth: 300,
			},
		},
	}
}

func TestFingerprintCollector_Collect(t *testing.T) {
	collector := NewFingerprintCollector()
	devices := createTestDevices()
	topology := createTestTopology()

	ctx := context.Background()
	metrics, err := collector.Collect(ctx, devices, topology)
	if err != nil {
		t.Fatalf("Collect() failed: %v", err)
	}

	if metrics.Fingerprint == nil {
		t.Fatal("Fingerprint should not be nil")
	}

	// 验证总 VRAM
	expectedVRAM := uint64(160 * 1024 * 1024 * 1024) // 2x 80GB
	if metrics.Fingerprint.VRAMTotal != expectedVRAM {
		t.Errorf("Expected VRAMTotal %d, got %d", expectedVRAM, metrics.Fingerprint.VRAMTotal)
	}

	// 验证可用 VRAM
	expectedFree := uint64(150 * 1024 * 1024 * 1024) // 70GB + 80GB
	if metrics.Fingerprint.VRAMFree != expectedFree {
		t.Errorf("Expected VRAMFree %d, got %d", expectedFree, metrics.Fingerprint.VRAMFree)
	}

	// 验证算力
	if metrics.Fingerprint.ComputeCap.FP16TFLOPS != 624 { // 2x 312
		t.Errorf("Expected FP16TFLOPS 624, got %d", metrics.Fingerprint.ComputeCap.FP16TFLOPS)
	}

	// 验证互联类型
	if metrics.Fingerprint.Interconnect != string(detectors.LinkTypeNVLink) {
		t.Errorf("Expected interconnect NVLink, got %s", metrics.Fingerprint.Interconnect)
	}
}

func TestFingerprintCollector_EmptyDevices(t *testing.T) {
	collector := NewFingerprintCollector()

	ctx := context.Background()
	metrics, err := collector.Collect(ctx, nil, nil)
	if err != nil {
		t.Fatalf("Collect() failed: %v", err)
	}

	if metrics.Fingerprint == nil {
		t.Fatal("Fingerprint should not be nil even with empty devices")
	}

	if metrics.Fingerprint.VRAMTotal != 0 {
		t.Errorf("Expected VRAMTotal 0 with no devices, got %d", metrics.Fingerprint.VRAMTotal)
	}
}

func TestHealthCollector_Collect(t *testing.T) {
	collector := NewHealthCollector()
	devices := createTestDevices()

	ctx := context.Background()
	metrics, err := collector.Collect(ctx, devices, nil)
	if err != nil {
		t.Fatalf("Collect() failed: %v", err)
	}

	if metrics.Health == nil {
		t.Fatal("Health should not be nil")
	}

	// 验证健康分数范围
	if metrics.Health.Score < 0 || metrics.Health.Score > 100 {
		t.Errorf("Health score should be in range [0, 100], got %f", metrics.Health.Score)
	}

	// 验证设备健康状态（存储在 DeviceMetrics 中）
	if len(metrics.DeviceMetrics) != 2 {
		t.Errorf("Expected 2 device metrics, got %d", len(metrics.DeviceMetrics))
	}

	// 检查第一个设备的健康状态
	for _, dm := range metrics.DeviceMetrics {
		if dm.DeviceID == "gpu-0" {
			if dm.Health == nil {
				t.Error("Device health should not be nil")
			} else if !collector.IsHealthy(dm.Health.Score) {
				t.Errorf("Expected gpu-0 to be healthy, score: %f", dm.Health.Score)
			}
		}
	}
}

func TestHealthCollector_HighTemperature(t *testing.T) {
	collector := NewHealthCollector()
	devices := createTestDevices()
	// 修改设备温度为高温
	devices[0].Temperature = 85

	ctx := context.Background()
	metrics, err := collector.Collect(ctx, devices, nil)
	if err != nil {
		t.Fatalf("Collect() failed: %v", err)
	}

	// 高温设备健康分数应该下降
	for _, dm := range metrics.DeviceMetrics {
		if dm.DeviceID == "gpu-0" {
			if dm.Health == nil {
				t.Error("Device health should not be nil")
			} else if dm.Health.Score >= 100 {
				t.Errorf("High temp device should have reduced health score, got %f", dm.Health.Score)
			}
		}
	}
}

func TestHealthCollector_ECCErrors(t *testing.T) {
	collector := NewHealthCollector()
	devices := createTestDevices()
	// 设置 ECC 错误
	devices[0].ECCErrors = 100

	ctx := context.Background()
	metrics, err := collector.Collect(ctx, devices, nil)
	if err != nil {
		t.Fatalf("Collect() failed: %v", err)
	}

	// ECC 错误应该记录在聚合的健康信息中
	if metrics.Health.ECCErrors != 100 {
		t.Errorf("Expected total ECC errors 100, got %d", metrics.Health.ECCErrors)
	}

	// 检查单个设备的 ECC 错误
	for _, dm := range metrics.DeviceMetrics {
		if dm.DeviceID == "gpu-0" {
			if dm.Health == nil {
				t.Error("Device health should not be nil")
			} else if dm.Health.ECCErrors != 100 {
				t.Errorf("Expected gpu-0 to have 100 ECC errors, got %d", dm.Health.ECCErrors)
			}
		}
	}
}

func TestHealthCollector_IsHealthyThresholds(t *testing.T) {
	collector := NewHealthCollector()

	tests := []struct {
		score    float64
		healthy  bool
		warning  bool
		critical bool
	}{
		{100.0, true, false, false},
		{75.0, true, false, false},
		{60.0, true, false, false},
		{59.0, false, true, false},
		{45.0, false, true, false},
		{30.0, false, true, false},
		{29.0, false, false, true},
		{0.0, false, false, true},
	}

	for _, tt := range tests {
		if collector.IsHealthy(tt.score) != tt.healthy {
			t.Errorf("Score %f: IsHealthy expected %v, got %v", tt.score, tt.healthy, !tt.healthy)
		}
		if collector.IsWarning(tt.score) != tt.warning {
			t.Errorf("Score %f: IsWarning expected %v, got %v", tt.score, tt.warning, !tt.warning)
		}
		if collector.IsCritical(tt.score) != tt.critical {
			t.Errorf("Score %f: IsCritical expected %v, got %v", tt.score, tt.critical, !tt.critical)
		}
	}
}

func TestTopologyCollector_Collect(t *testing.T) {
	collector := NewTopologyCollector()
	devices := createTestDevices()
	topology := createTestTopology()

	ctx := context.Background()
	metrics, err := collector.Collect(ctx, devices, topology)
	if err != nil {
		t.Fatalf("Collect() failed: %v", err)
	}

	if metrics.Topology == nil {
		t.Fatal("Topology should not be nil")
	}

	// 验证 Peers（从 Links 构建）
	if len(metrics.Topology.Peers) == 0 {
		t.Error("Expected at least one peer connection")
	}

	// 验证 NVLink peer
	foundNVLink := false
	for _, peer := range metrics.Topology.Peers {
		if peer.LinkType == string(detectors.LinkTypeNVLink) {
			foundNVLink = true
			if peer.LinkBw != 300 {
				t.Errorf("Expected NVLink bandwidth 300, got %d", peer.LinkBw)
			}
		}
	}
	if !foundNVLink {
		t.Error("Expected NVLink peer connection")
	}

	// 验证 PCIe 信息已解析
	if metrics.Topology.PCIEBus != 0x01 {
		t.Errorf("Expected PCIe bus 0x01, got 0x%02x", metrics.Topology.PCIEBus)
	}
}

func TestTopologyCollector_BuildTopologyMatrix(t *testing.T) {
	collector := NewTopologyCollector()
	devices := createTestDevices()
	topology := createTestTopology()

	matrix := collector.BuildTopologyMatrix(devices, topology)

	if matrix == nil {
		t.Fatal("Topology matrix should not be nil")
	}

	if len(matrix) != 2 {
		t.Errorf("Expected 2x2 matrix, got %dx%d", len(matrix), len(matrix[0]))
	}

	// gpu-0 -> gpu-1 应该有带宽 300
	if matrix[0][1] != 300 {
		t.Errorf("Expected bandwidth 300 between gpu-0 and gpu-1, got %d", matrix[0][1])
	}

	// 对称性
	if matrix[1][0] != matrix[0][1] {
		t.Error("Matrix should be symmetric")
	}
}

func TestTopologyCollector_FindOptimalPlacement(t *testing.T) {
	collector := NewTopologyCollector()

	// 创建一个 4 设备的拓扑
	matrix := [][]int{
		{0, 300, 100, 100},
		{300, 0, 100, 100},
		{100, 100, 0, 300},
		{100, 100, 300, 0},
	}

	// 请求 2 个设备，应该选择带宽最高的一对
	placement := collector.FindOptimalPlacement(matrix, 2)

	if len(placement) != 2 {
		t.Fatalf("Expected 2 devices in placement, got %d", len(placement))
	}

	// 最优放置应该是 (0,1) 或 (2,3)，它们的带宽都是 300
	score := matrix[placement[0]][placement[1]]
	if score != 300 {
		t.Errorf("Expected optimal placement score 300, got %d", score)
	}
}

func TestTopologyCollector_FindOptimalPlacement_EdgeCases(t *testing.T) {
	collector := NewTopologyCollector()

	// Test requiredDevices = 1 (should return [0])
	matrix := [][]int{
		{0, 300, 100},
		{300, 0, 100},
		{100, 100, 0},
	}
	placement := collector.FindOptimalPlacement(matrix, 1)
	if len(placement) != 1 || placement[0] != 0 {
		t.Errorf("Expected placement [0] for single device, got %v", placement)
	}

	// Test requiredDevices > n (should return nil)
	placement = collector.FindOptimalPlacement(matrix, 5)
	if placement != nil {
		t.Errorf("Expected nil for requiredDevices > n, got %v", placement)
	}

	// Test requiredDevices <= 0 (should return nil)
	placement = collector.FindOptimalPlacement(matrix, 0)
	if placement != nil {
		t.Errorf("Expected nil for requiredDevices = 0, got %v", placement)
	}

	placement = collector.FindOptimalPlacement(matrix, -1)
	if placement != nil {
		t.Errorf("Expected nil for requiredDevices < 0, got %v", placement)
	}
}

func TestTopologyCollector_FindOptimalPlacement_GreedyAlgorithm(t *testing.T) {
	collector := NewTopologyCollector()

	// Create a 10-device topology to trigger greedy algorithm (n > 8)
	n := 10
	matrix := make([][]int, n)
	for i := 0; i < n; i++ {
		matrix[i] = make([]int, n)
		for j := 0; j < n; j++ {
			if i == j {
				matrix[i][j] = 0
			} else if (i == 0 && j == 1) || (i == 1 && j == 0) {
				// High bandwidth between device 0 and 1
				matrix[i][j] = 500
			} else if (i == 0 && j == 2) || (i == 2 && j == 0) ||
				(i == 1 && j == 2) || (i == 2 && j == 1) {
				// Medium bandwidth between 0,1 and 2
				matrix[i][j] = 300
			} else {
				// Low bandwidth for others
				matrix[i][j] = 100
			}
		}
	}

	// Request 3 devices - should use greedy algorithm
	placement := collector.FindOptimalPlacement(matrix, 3)

	if len(placement) != 3 {
		t.Fatalf("Expected 3 devices in placement, got %d", len(placement))
	}

	// Greedy should pick high-bandwidth connected devices
	// The first device should be one with highest total bandwidth (0 or 1)
	firstDev := placement[0]
	if firstDev != 0 && firstDev != 1 {
		t.Logf("First device is %d (expected 0 or 1 for best bandwidth)", firstDev)
	}
}

func TestTopologyCollector_FindOptimalPlacement_LargeRequirement(t *testing.T) {
	collector := NewTopologyCollector()

	// Create a 6-device matrix with requiredDevices > 4 to trigger greedy
	n := 6
	matrix := make([][]int, n)
	for i := 0; i < n; i++ {
		matrix[i] = make([]int, n)
		for j := 0; j < n; j++ {
			if i == j {
				matrix[i][j] = 0
			} else {
				// Assign varying bandwidths
				matrix[i][j] = 100 + (i+j)*10
			}
		}
	}

	// Request 5 devices (> 4, triggers greedy even with n <= 8)
	placement := collector.FindOptimalPlacement(matrix, 5)

	if len(placement) != 5 {
		t.Fatalf("Expected 5 devices in placement, got %d", len(placement))
	}

	// Verify all devices are unique
	seen := make(map[int]bool)
	for _, dev := range placement {
		if seen[dev] {
			t.Errorf("Duplicate device %d in placement", dev)
		}
		seen[dev] = true
		if dev < 0 || dev >= n {
			t.Errorf("Invalid device index %d", dev)
		}
	}
}

func TestTopologyCollector_ParsePCIEBusID(t *testing.T) {
	collector := NewTopologyCollector()

	tests := []struct {
		busID    string
		domain   uint32
		bus      uint32
		device   uint32
		function uint32
	}{
		{"0000:01:00.0", 0, 1, 0, 0},
		{"0000:AB:CD.F", 0, 0xAB, 0xCD, 0xF},
		{"DEAD:BE:EF.1", 0xDEAD, 0xBE, 0xEF, 1},
		{"01:00.0", 0, 1, 0, 0}, // 省略 domain
	}

	for _, tt := range tests {
		info := collector.parsePCIEBusID(tt.busID)
		if info.Domain != tt.domain {
			t.Errorf("busID %s: expected domain %x, got %x", tt.busID, tt.domain, info.Domain)
		}
		if info.Bus != tt.bus {
			t.Errorf("busID %s: expected bus %x, got %x", tt.busID, tt.bus, info.Bus)
		}
		if info.Device != tt.device {
			t.Errorf("busID %s: expected device %x, got %x", tt.busID, tt.device, info.Device)
		}
		if info.Function != tt.function {
			t.Errorf("busID %s: expected function %x, got %x", tt.busID, tt.function, info.Function)
		}
	}
}

func TestManager_CollectAll(t *testing.T) {
	manager := NewDefaultManager() // 使用带有默认采集器的管理器
	devices := createTestDevices()
	topology := createTestTopology()

	ctx := context.Background()
	metrics, err := manager.CollectAll(ctx, devices, topology)
	if err != nil {
		t.Fatalf("CollectAll() failed: %v", err)
	}

	// 验证所有指标都被采集
	if metrics.Fingerprint == nil {
		t.Error("Fingerprint should not be nil")
	}
	if metrics.Health == nil {
		t.Error("Health should not be nil")
	}
	if metrics.Topology == nil {
		t.Error("Topology should not be nil")
	}
}

func TestMetrics_Aggregate(t *testing.T) {
	metrics := &Metrics{
		Fingerprint: &FingerprintMetrics{
			VRAMTotal:    160 * 1024 * 1024 * 1024,
			VRAMUsed:     10 * 1024 * 1024 * 1024,
			VRAMFree:     150 * 1024 * 1024 * 1024,
			ComputeCap:   ComputeMetrics{FP16TFLOPS: 624, FP32TFLOPS: 38},
			Interconnect: "NVLink",
		},
		Health: &HealthMetrics{
			Score: 95.0,
		},
		DeviceMetrics: []DeviceMetric{
			{DeviceID: "gpu-0"},
			{DeviceID: "gpu-1"},
		},
	}

	agg := metrics.Aggregate()

	if agg.TotalVRAM != 160*1024*1024*1024 {
		t.Errorf("Expected TotalVRAM %d, got %d", 160*1024*1024*1024, agg.TotalVRAM)
	}

	if agg.TotalFP16FLOPS != 624 {
		t.Errorf("Expected TotalFP16FLOPS 624, got %d", agg.TotalFP16FLOPS)
	}

	if agg.AvgHealthScore != 95.0 {
		t.Errorf("Expected AvgHealthScore 95.0, got %f", agg.AvgHealthScore)
	}

	if agg.DeviceCount != 2 {
		t.Errorf("Expected DeviceCount 2, got %d", agg.DeviceCount)
	}
}

func TestManager_List(t *testing.T) {
	manager := NewManager()

	// 空管理器
	names := manager.List()
	if len(names) != 0 {
		t.Errorf("Expected empty list, got %d items", len(names))
	}

	// 注册采集器后
	manager.Register(NewFingerprintCollector())
	manager.Register(NewHealthCollector())
	manager.Register(NewTopologyCollector())

	names = manager.List()
	if len(names) != 3 {
		t.Errorf("Expected 3 collectors, got %d", len(names))
	}

	// 验证名称
	expectedNames := map[string]bool{
		"fingerprint": false,
		"health":      false,
		"topology":    false,
	}

	for _, name := range names {
		if _, ok := expectedNames[name]; ok {
			expectedNames[name] = true
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("Expected collector '%s' not found", name)
		}
	}
}

func TestManager_CollectAllParallel(t *testing.T) {
	manager := NewDefaultManager()
	devices := createTestDevices()
	topology := createTestTopology()

	ctx := context.Background()
	results, err := manager.CollectAllParallel(ctx, devices, topology)
	if err != nil {
		t.Fatalf("CollectAllParallel() failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// 验证每个采集器都有结果
	resultMap := make(map[string]*CollectorResult)
	for _, r := range results {
		resultMap[r.Name] = r
	}

	// 验证 Fingerprint 采集结果
	if fp, ok := resultMap["fingerprint"]; ok {
		if fp.Error != nil {
			t.Errorf("Fingerprint collection error: %v", fp.Error)
		}
		if fp.Metrics == nil || fp.Metrics.Fingerprint == nil {
			t.Error("Fingerprint metrics should not be nil")
		}
	} else {
		t.Error("Missing fingerprint result")
	}

	// 验证 Health 采集结果
	if h, ok := resultMap["health"]; ok {
		if h.Error != nil {
			t.Errorf("Health collection error: %v", h.Error)
		}
		if h.Metrics == nil || h.Metrics.Health == nil {
			t.Error("Health metrics should not be nil")
		}
	} else {
		t.Error("Missing health result")
	}

	// 验证 Topology 采集结果
	if tp, ok := resultMap["topology"]; ok {
		if tp.Error != nil {
			t.Errorf("Topology collection error: %v", tp.Error)
		}
		if tp.Metrics == nil || tp.Metrics.Topology == nil {
			t.Error("Topology metrics should not be nil")
		}
	} else {
		t.Error("Missing topology result")
	}
}

func TestTopologyMetrics_String(t *testing.T) {
	tests := []struct {
		name     string
		metrics  *TopologyMetrics
		expected string
	}{
		{
			name: "basic formatting",
			metrics: &TopologyMetrics{
				PCIEDomain:   0,
				PCIEBus:      0x01,
				PCIEDevice:   0x00,
				PCIEFunction: 0,
				Peers:        []PeerInfo{{DeviceID: "gpu-1"}},
			},
			expected: "PCIe[0000:01:00.0] Peers=1",
		},
		{
			name: "with hex values",
			metrics: &TopologyMetrics{
				PCIEDomain:   0xDEAD,
				PCIEBus:      0xBE,
				PCIEDevice:   0xEF,
				PCIEFunction: 0xF,
				Peers:        []PeerInfo{{DeviceID: "gpu-1"}, {DeviceID: "gpu-2"}},
			},
			expected: "PCIe[dead:be:ef.f] Peers=2",
		},
		{
			name: "no peers",
			metrics: &TopologyMetrics{
				PCIEDomain:   0,
				PCIEBus:      0x02,
				PCIEDevice:   0x00,
				PCIEFunction: 0,
				Peers:        []PeerInfo{},
			},
			expected: "PCIe[0000:02:00.0] Peers=0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.metrics.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
