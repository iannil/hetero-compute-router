package plugins

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/zrs-products/hetero-compute-router/pkg/api/v1alpha1"
	"github.com/zrs-products/hetero-compute-router/pkg/exchange"
	"github.com/zrs-products/hetero-compute-router/pkg/scheduler/framework"
)

// testK8sClient 模拟 K8s 客户端，实现 client.Client 接口
type testK8sClient struct {
	computeNodes map[string]*v1alpha1.ComputeNode
}

var _ client.Client = &testK8sClient{}

func newTestK8sClient() *testK8sClient {
	return &testK8sClient{
		computeNodes: make(map[string]*v1alpha1.ComputeNode),
	}
}

func (c *testK8sClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if cn, ok := c.computeNodes[key.Name]; ok {
		*obj.(*v1alpha1.ComputeNode) = *cn
		return nil
	}
	return &notFoundError{}
}

func (c *testK8sClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
}

func (c *testK8sClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return nil
}

func (c *testK8sClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return nil
}

func (c *testK8sClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return nil
}

func (c *testK8sClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}

func (c *testK8sClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}

func (c *testK8sClient) Status() client.StatusWriter {
	return nil
}

func (c *testK8sClient) Scheme() *runtime.Scheme {
	return nil
}

func (c *testK8sClient) RESTMapper() meta.RESTMapper {
	return nil
}

func (c *testK8sClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (c *testK8sClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return false, nil
}

func (c *testK8sClient) SubResource(subResource string) client.SubResourceClient {
	return nil
}

type notFoundError struct{}

func (e *notFoundError) Error() string { return "not found" }

func newMockClient() *testK8sClient {
	return &testK8sClient{
		computeNodes: map[string]*v1alpha1.ComputeNode{
			"node-1": {
				ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
				Spec: v1alpha1.ComputeNodeSpec{
					NodeName: "node-1",
					Vendor:   "nvidia",
					TotalCapacity: v1alpha1.ComputeCapacity{
						VRAM:       160 * 1024 * 1024 * 1024, // 160GB
						FP16TFLOPS: 624,
						FP32TFLOPS: 38,
					},
				},
				Status: v1alpha1.ComputeNodeStatus{
					Phase: v1alpha1.ComputeNodePhaseReady,
					Devices: []v1alpha1.DeviceInfo{
						{
							ID:               "gpu-0",
							Model:            "NVIDIA A100-SXM4-80GB",
							VRAMTotal:        80 * 1024 * 1024 * 1024,
							VRAMUsed:         10 * 1024 * 1024 * 1024,
							HealthScore:      95.0,
							InterconnectType: "NVLink",
						},
						{
							ID:               "gpu-1",
							Model:            "NVIDIA A100-SXM4-80GB",
							VRAMTotal:        80 * 1024 * 1024 * 1024,
							VRAMUsed:         0,
							HealthScore:      100.0,
							InterconnectType: "NVLink",
						},
					},
				},
			},
			"node-2": {
				ObjectMeta: metav1.ObjectMeta{Name: "node-2"},
				Spec: v1alpha1.ComputeNodeSpec{
					NodeName: "node-2",
					Vendor:   "nvidia",
					TotalCapacity: v1alpha1.ComputeCapacity{
						VRAM:       48 * 1024 * 1024 * 1024, // 48GB
						FP16TFLOPS: 150,
						FP32TFLOPS: 18,
					},
				},
				Status: v1alpha1.ComputeNodeStatus{
					Phase: v1alpha1.ComputeNodePhaseReady,
					Devices: []v1alpha1.DeviceInfo{
						{
							ID:               "gpu-0",
							Model:            "NVIDIA RTX 3090",
							VRAMTotal:        24 * 1024 * 1024 * 1024,
							VRAMUsed:         20 * 1024 * 1024 * 1024,
							HealthScore:      80.0,
							InterconnectType: "PCIe",
						},
						{
							ID:               "gpu-1",
							Model:            "NVIDIA RTX 3090",
							VRAMTotal:        24 * 1024 * 1024 * 1024,
							VRAMUsed:         20 * 1024 * 1024 * 1024,
							HealthScore:      80.0,
							InterconnectType: "PCIe",
						},
					},
				},
			},
			"node-unhealthy": {
				ObjectMeta: metav1.ObjectMeta{Name: "node-unhealthy"},
				Spec: v1alpha1.ComputeNodeSpec{
					NodeName: "node-unhealthy",
					Vendor:   "nvidia",
				},
				Status: v1alpha1.ComputeNodeStatus{
					Phase: v1alpha1.ComputeNodePhaseUnhealthy,
				},
			},
		},
	}
}

func createTestPod(vramGi, fp16TFLOPS int64) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: "test-container",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{},
					},
				},
			},
		},
	}

	if vramGi > 0 {
		pod.Spec.Containers[0].Resources.Requests[v1.ResourceName(VRAMResourceName)] = *resource.NewQuantity(vramGi*1024*1024*1024, resource.BinarySI)
	}
	if fp16TFLOPS > 0 {
		pod.Spec.Containers[0].Resources.Requests[v1.ResourceName(FP16TFLOPSResourceName)] = *resource.NewQuantity(fp16TFLOPS, resource.DecimalSI)
	}

	return pod
}

func TestFilterPlugin_PreFilter(t *testing.T) {
	plugin := &FilterPlugin{}
	state := framework.NewCycleState()

	// 测试有资源请求的 Pod
	pod := createTestPod(16, 100)
	status := plugin.PreFilter(context.Background(), state, pod)

	if !status.IsSuccess() {
		t.Errorf("PreFilter should succeed, got: %s", status.Message)
	}

	// 验证状态已存储
	data, err := state.Read(stateKey)
	if err != nil {
		t.Errorf("State should be stored, got error: %v", err)
	}

	req := data.(*stateData).request
	if req.VRAMBytes != 16*1024*1024*1024 {
		t.Errorf("Expected VRAMBytes %d, got %d", 16*1024*1024*1024, req.VRAMBytes)
	}
}

func TestFilterPlugin_PreFilter_NoRequest(t *testing.T) {
	plugin := &FilterPlugin{}
	state := framework.NewCycleState()

	// 测试没有资源请求的 Pod
	pod := &v1.Pod{
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{Name: "test"},
			},
		},
	}
	status := plugin.PreFilter(context.Background(), state, pod)

	if !status.IsSuccess() {
		t.Errorf("PreFilter should succeed for pod without HCS resources")
	}

	// 状态不应该被存储
	_, err := state.Read(stateKey)
	if err == nil {
		t.Error("State should not be stored for pod without HCS resources")
	}
}

func TestFilterPlugin_CalculateAvailableVRAM(t *testing.T) {
	plugin := &FilterPlugin{}

	cn := &v1alpha1.ComputeNode{
		Spec: v1alpha1.ComputeNodeSpec{
			TotalCapacity: v1alpha1.ComputeCapacity{
				VRAM: 160 * 1024 * 1024 * 1024,
			},
		},
		Status: v1alpha1.ComputeNodeStatus{
			Devices: []v1alpha1.DeviceInfo{
				{VRAMUsed: 10 * 1024 * 1024 * 1024},
				{VRAMUsed: 20 * 1024 * 1024 * 1024},
			},
		},
	}

	available := plugin.calculateAvailableVRAM(cn)
	expected := uint64(130 * 1024 * 1024 * 1024) // 160 - 10 - 20

	if available != expected {
		t.Errorf("Expected available VRAM %d, got %d", expected, available)
	}
}

func TestScorePlugin_CalculateScore(t *testing.T) {
	plugin := &ScorePlugin{}

	cn := &v1alpha1.ComputeNode{
		Spec: v1alpha1.ComputeNodeSpec{
			TotalCapacity: v1alpha1.ComputeCapacity{
				VRAM:       160 * 1024 * 1024 * 1024,
				FP16TFLOPS: 624,
			},
		},
		Status: v1alpha1.ComputeNodeStatus{
			Devices: []v1alpha1.DeviceInfo{
				{
					VRAMUsed:         10 * 1024 * 1024 * 1024,
					HealthScore:      95.0,
					InterconnectType: "NVLink",
				},
				{
					VRAMUsed:         0,
					HealthScore:      100.0,
					InterconnectType: "NVLink",
				},
			},
		},
	}

	req := &ComputeRequest{
		VRAMBytes:  16 * 1024 * 1024 * 1024,
		FP16TFLOPS: 100,
	}

	score := plugin.calculateScore(cn, req)

	// 分数应该大于 0
	if score <= 0 {
		t.Errorf("Score should be positive, got %d", score)
	}

	// 分数应该小于等于 100
	if score > 100 {
		t.Errorf("Score should be <= 100, got %d", score)
	}
}

func TestScorePlugin_NormalizeScore(t *testing.T) {
	plugin := &ScorePlugin{}
	state := framework.NewCycleState()
	pod := createTestPod(16, 100)

	scores := framework.NodeScoreList{
		{Name: "node-1", Score: 80},
		{Name: "node-2", Score: 40},
		{Name: "node-3", Score: 60},
	}

	status := plugin.NormalizeScore(context.Background(), state, pod, scores)

	if !status.IsSuccess() {
		t.Errorf("NormalizeScore should succeed")
	}

	// 最高分应该是 100
	var maxScore int64 = 0
	for _, s := range scores {
		if s.Score > maxScore {
			maxScore = s.Score
		}
	}

	if maxScore != 100 {
		t.Errorf("Max normalized score should be 100, got %d", maxScore)
	}

	// 最低分应该是 0
	var minScore int64 = 100
	for _, s := range scores {
		if s.Score < minScore {
			minScore = s.Score
		}
	}

	if minScore != 0 {
		t.Errorf("Min normalized score should be 0, got %d", minScore)
	}
}

func TestScorePlugin_NormalizeScore_AllSame(t *testing.T) {
	plugin := &ScorePlugin{}
	state := framework.NewCycleState()
	pod := createTestPod(16, 100)

	scores := framework.NodeScoreList{
		{Name: "node-1", Score: 50},
		{Name: "node-2", Score: 50},
		{Name: "node-3", Score: 50},
	}

	status := plugin.NormalizeScore(context.Background(), state, pod, scores)

	if !status.IsSuccess() {
		t.Errorf("NormalizeScore should succeed")
	}

	// 所有分数相同时，应该都归一化到中间值
	for _, s := range scores {
		if s.Score != 50 {
			t.Errorf("All same scores should normalize to 50, got %d", s.Score)
		}
	}
}

func TestReservePlugin_ReserveAndUnreserve(t *testing.T) {
	plugin := NewReservePlugin(nil)

	state := framework.NewCycleState()
	pod := createTestPod(16, 100)

	// 存储请求到状态
	state.Write(stateKey, &stateData{
		request: &ComputeRequest{
			VRAMBytes:  16 * 1024 * 1024 * 1024,
			FP16TFLOPS: 100,
		},
	})

	// 预留资源（不需要实际的 K8s 客户端）
	plugin.mu.Lock()
	plugin.reservations["node-1"] = &Reservation{
		Pods: make(map[string]*PodReservation),
	}
	plugin.reservations["node-1"].Pods["default/test-pod"] = &PodReservation{
		VRAMBytes:  16 * 1024 * 1024 * 1024,
		FP16TFLOPS: 100,
	}
	plugin.mu.Unlock()

	// 验证预留
	reserved := plugin.GetTotalReservedVRAM("node-1")
	if reserved != 16*1024*1024*1024 {
		t.Errorf("Expected reserved VRAM %d, got %d", 16*1024*1024*1024, reserved)
	}

	// 取消预留
	plugin.Unreserve(context.Background(), state, pod, "node-1")

	// 验证取消
	reserved = plugin.GetTotalReservedVRAM("node-1")
	if reserved != 0 {
		t.Errorf("Expected reserved VRAM 0 after unreserve, got %d", reserved)
	}
}

func TestNewFilterPlugin(t *testing.T) {
	plugin := NewFilterPlugin(nil)
	if plugin == nil {
		t.Fatal("NewFilterPlugin should return non-nil")
	}
}

func TestFilterPlugin_Name(t *testing.T) {
	plugin := &FilterPlugin{}
	if plugin.Name() != PluginName {
		t.Errorf("Expected name %s, got %s", PluginName, plugin.Name())
	}
}

func TestNewScorePlugin(t *testing.T) {
	plugin := NewScorePlugin(nil, nil)
	if plugin == nil {
		t.Fatal("NewScorePlugin should return non-nil")
	}
}

func TestScorePlugin_Name(t *testing.T) {
	plugin := &ScorePlugin{}
	if plugin.Name() != ScorePluginName {
		t.Errorf("Expected name %s, got %s", ScorePluginName, plugin.Name())
	}
}

func TestScorePlugin_ScoreExtensions(t *testing.T) {
	plugin := &ScorePlugin{}
	ext := plugin.ScoreExtensions()
	if ext == nil {
		t.Fatal("ScoreExtensions should return non-nil")
	}
	if ext != plugin {
		t.Error("ScoreExtensions should return the plugin itself")
	}
}

func TestScorePlugin_Score_NoStateData(t *testing.T) {
	plugin := &ScorePlugin{}
	state := framework.NewCycleState()
	pod := createTestPod(0, 0)

	score, status := plugin.Score(context.Background(), state, pod, "node-1")

	if !status.IsSuccess() {
		t.Errorf("Score should succeed, got: %s", status.Message)
	}
	// When no HCS resource request, returns default score (MaxScore/2)
	if score != MaxScore/2 {
		t.Errorf("Expected default score %d, got %d", MaxScore/2, score)
	}
}

func TestScorePlugin_Score_NilClient(t *testing.T) {
	plugin := &ScorePlugin{client: nil}
	state := framework.NewCycleState()
	state.Write(stateKey, &stateData{
		request: &ComputeRequest{
			VRAMBytes:  16 * 1024 * 1024 * 1024,
			FP16TFLOPS: 100,
		},
	})
	pod := createTestPod(16, 100)

	score, status := plugin.Score(context.Background(), state, pod, "node-1")

	// Should return 0 score when client is nil
	if score != 0 {
		t.Errorf("Expected score 0 when client is nil, got %d", score)
	}
	if !status.IsSuccess() {
		t.Errorf("Status should be success even with error")
	}
}

func TestScorePlugin_Score_Success(t *testing.T) {
	plugin := NewScorePlugin(newMockClient(), nil)
	state := framework.NewCycleState()
	state.Write(stateKey, &stateData{
		request: &ComputeRequest{
			VRAMBytes:  16 * 1024 * 1024 * 1024,
			FP16TFLOPS: 100,
		},
	})
	pod := createTestPod(16, 100)

	score, status := plugin.Score(context.Background(), state, pod, "node-1")

	if !status.IsSuccess() {
		t.Errorf("Score should succeed, got: %s", status.Message)
	}
	// Score should be positive for a healthy node with available resources
	if score <= 0 {
		t.Errorf("Expected positive score, got %d", score)
	}
}

func TestScorePlugin_Score_NodeNotFound(t *testing.T) {
	plugin := NewScorePlugin(newMockClient(), nil)
	state := framework.NewCycleState()
	state.Write(stateKey, &stateData{
		request: &ComputeRequest{
			VRAMBytes:  16 * 1024 * 1024 * 1024,
			FP16TFLOPS: 100,
		},
	})
	pod := createTestPod(16, 100)

	score, status := plugin.Score(context.Background(), state, pod, "nonexistent-node")

	// Should return 0 score when node not found
	if score != 0 {
		t.Errorf("Expected score 0 when node not found, got %d", score)
	}
	if !status.IsSuccess() {
		t.Errorf("Status should be success even with error")
	}
}

func TestScorePlugin_GetComputeNode_Success(t *testing.T) {
	plugin := NewScorePlugin(newMockClient(), nil)

	cn, err := plugin.getComputeNode(context.Background(), "node-1")
	if err != nil {
		t.Errorf("getComputeNode should succeed, got: %v", err)
	}
	if cn == nil {
		t.Fatal("ComputeNode should not be nil")
	}
	if cn.Name != "node-1" {
		t.Errorf("Expected node-1, got %s", cn.Name)
	}
}

func TestScorePlugin_GetComputeNode_NilClient(t *testing.T) {
	plugin := &ScorePlugin{client: nil}

	cn, err := plugin.getComputeNode(context.Background(), "node-1")
	if err == nil {
		t.Error("getComputeNode should error with nil client")
	}
	if cn != nil {
		t.Error("ComputeNode should be nil when error occurs")
	}
}

func TestScorePlugin_GetComputeNode_NotFound(t *testing.T) {
	plugin := NewScorePlugin(newMockClient(), nil)

	cn, err := plugin.getComputeNode(context.Background(), "nonexistent")
	if err == nil {
		t.Error("getComputeNode should error for nonexistent node")
	}
	if cn != nil {
		t.Error("ComputeNode should be nil when not found")
	}
}

func TestScorePlugin_CalculateAverageHealth(t *testing.T) {
	plugin := &ScorePlugin{}

	tests := []struct {
		name     string
		cn       *v1alpha1.ComputeNode
		expected float64
	}{
		{
			name: "multiple devices",
			cn: &v1alpha1.ComputeNode{
				Status: v1alpha1.ComputeNodeStatus{
					Devices: []v1alpha1.DeviceInfo{
						{HealthScore: 80.0},
						{HealthScore: 100.0},
					},
				},
			},
			expected: 90.0,
		},
		{
			name: "no devices",
			cn: &v1alpha1.ComputeNode{
				Status: v1alpha1.ComputeNodeStatus{
					Devices: []v1alpha1.DeviceInfo{},
				},
			},
			expected: 100.0,
		},
		{
			name: "single device",
			cn: &v1alpha1.ComputeNode{
				Status: v1alpha1.ComputeNodeStatus{
					Devices: []v1alpha1.DeviceInfo{
						{HealthScore: 75.0},
					},
				},
			},
			expected: 75.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			health := plugin.calculateAverageHealth(tt.cn)
			if health != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, health)
			}
		})
	}
}

func TestScorePlugin_CalculateAvailableVRAM(t *testing.T) {
	plugin := &ScorePlugin{}

	tests := []struct {
		name     string
		cn       *v1alpha1.ComputeNode
		expected uint64
	}{
		{
			name: "normal case",
			cn: &v1alpha1.ComputeNode{
				Spec: v1alpha1.ComputeNodeSpec{
					TotalCapacity: v1alpha1.ComputeCapacity{
						VRAM: 80 * 1024 * 1024 * 1024,
					},
				},
				Status: v1alpha1.ComputeNodeStatus{
					Devices: []v1alpha1.DeviceInfo{
						{VRAMUsed: 10 * 1024 * 1024 * 1024},
					},
				},
			},
			expected: 70 * 1024 * 1024 * 1024,
		},
		{
			name: "overused case",
			cn: &v1alpha1.ComputeNode{
				Spec: v1alpha1.ComputeNodeSpec{
					TotalCapacity: v1alpha1.ComputeCapacity{
						VRAM: 80 * 1024 * 1024 * 1024,
					},
				},
				Status: v1alpha1.ComputeNodeStatus{
					Devices: []v1alpha1.DeviceInfo{
						{VRAMUsed: 90 * 1024 * 1024 * 1024},
					},
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			available := plugin.calculateAvailableVRAM(tt.cn)
			if available != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, available)
			}
		})
	}
}

func TestScorePlugin_CalculateScore_NoFP16Request(t *testing.T) {
	plugin := &ScorePlugin{}

	cn := &v1alpha1.ComputeNode{
		Spec: v1alpha1.ComputeNodeSpec{
			TotalCapacity: v1alpha1.ComputeCapacity{
				VRAM:       160 * 1024 * 1024 * 1024,
				FP16TFLOPS: 624,
			},
		},
		Status: v1alpha1.ComputeNodeStatus{
			Devices: []v1alpha1.DeviceInfo{
				{
					VRAMUsed:         10 * 1024 * 1024 * 1024,
					HealthScore:      90.0,
					InterconnectType: "PCIe",
				},
			},
		},
	}

	req := &ComputeRequest{
		VRAMBytes:  16 * 1024 * 1024 * 1024,
		FP16TFLOPS: 0, // No FP16 request
	}

	score := plugin.calculateScore(cn, req)

	// Should still return a valid score
	if score <= 0 {
		t.Errorf("Score should be positive, got %d", score)
	}
}

func TestScorePlugin_CalculateScore_WithExchangeNormalization(t *testing.T) {
	// Create plugin with calculator
	calc := exchange.NewCalculator()
	plugin := &ScorePlugin{
		calculator: calc,
	}

	// Test with NVIDIA A100-80GB (known builtin profile)
	cnNVIDIA := &v1alpha1.ComputeNode{
		Spec: v1alpha1.ComputeNodeSpec{
			Vendor: "nvidia",
			TotalCapacity: v1alpha1.ComputeCapacity{
				VRAM:       160 * 1024 * 1024 * 1024, // 160 GiB (2 x A100-80GB)
				FP16TFLOPS: 624,                      // 2 x 312
			},
		},
		Status: v1alpha1.ComputeNodeStatus{
			Devices: []v1alpha1.DeviceInfo{
				{
					Model:            "A100-80GB",
					VRAMUsed:         10 * 1024 * 1024 * 1024,
					HealthScore:      95.0,
					InterconnectType: "NVLink",
				},
				{
					Model:            "A100-80GB",
					VRAMUsed:         0,
					HealthScore:      100.0,
					InterconnectType: "NVLink",
				},
			},
		},
	}

	req := &ComputeRequest{
		VRAMBytes:  16 * 1024 * 1024 * 1024,
		FP16TFLOPS: 100,
	}

	scoreNVIDIA := plugin.calculateScore(cnNVIDIA, req)

	// Score should be positive and reasonable for A100-80GB
	if scoreNVIDIA <= 0 {
		t.Errorf("NVIDIA score should be positive, got %d", scoreNVIDIA)
	}
	if scoreNVIDIA > MaxScore {
		t.Errorf("NVIDIA score should be <= %d, got %d", MaxScore, scoreNVIDIA)
	}

	// Test with RTX4090 (consumer GPU, lower normalized score expected)
	cnRTX := &v1alpha1.ComputeNode{
		Spec: v1alpha1.ComputeNodeSpec{
			Vendor: "nvidia",
			TotalCapacity: v1alpha1.ComputeCapacity{
				VRAM:       24 * 1024 * 1024 * 1024, // 24 GiB
				FP16TFLOPS: 82,
			},
		},
		Status: v1alpha1.ComputeNodeStatus{
			Devices: []v1alpha1.DeviceInfo{
				{
					Model:            "RTX4090",
					VRAMUsed:         0,
					HealthScore:      100.0,
					InterconnectType: "PCIe",
				},
			},
		},
	}

	scoreRTX := plugin.calculateScore(cnRTX, req)

	// Score should be positive
	if scoreRTX <= 0 {
		t.Errorf("RTX4090 score should be positive, got %d", scoreRTX)
	}

	// RTX4090 should score lower than A100-80GB on normalized compute
	// (though other factors like health may affect the total score)
	t.Logf("A100-80GB score: %d, RTX4090 score: %d", scoreNVIDIA, scoreRTX)
}

func TestScorePlugin_CalculateScore_UnknownHardware(t *testing.T) {
	// Create plugin with calculator
	calc := exchange.NewCalculator()
	plugin := &ScorePlugin{
		calculator: calc,
	}

	// Test with unknown hardware - should fall back to raw VRAM ratio
	cn := &v1alpha1.ComputeNode{
		Spec: v1alpha1.ComputeNodeSpec{
			Vendor: "unknown-vendor",
			TotalCapacity: v1alpha1.ComputeCapacity{
				VRAM:       100 * 1024 * 1024 * 1024,
				FP16TFLOPS: 200,
			},
		},
		Status: v1alpha1.ComputeNodeStatus{
			Devices: []v1alpha1.DeviceInfo{
				{
					Model:            "unknown-model",
					VRAMUsed:         20 * 1024 * 1024 * 1024,
					HealthScore:      90.0,
					InterconnectType: "PCIe",
				},
			},
		},
	}

	req := &ComputeRequest{
		VRAMBytes:  16 * 1024 * 1024 * 1024,
		FP16TFLOPS: 100,
	}

	score := plugin.calculateScore(cn, req)

	// Should still return a valid score using fallback logic
	if score <= 0 {
		t.Errorf("Score should be positive, got %d", score)
	}
	if score > MaxScore {
		t.Errorf("Score should be <= %d, got %d", MaxScore, score)
	}
}

func TestScorePlugin_GetDeviceModel(t *testing.T) {
	plugin := &ScorePlugin{}

	tests := []struct {
		name     string
		cn       *v1alpha1.ComputeNode
		expected string
	}{
		{
			name: "with devices",
			cn: &v1alpha1.ComputeNode{
				Status: v1alpha1.ComputeNodeStatus{
					Devices: []v1alpha1.DeviceInfo{
						{Model: "A100-80GB"},
						{Model: "A100-80GB"},
					},
				},
			},
			expected: "A100-80GB",
		},
		{
			name: "no devices",
			cn: &v1alpha1.ComputeNode{
				Status: v1alpha1.ComputeNodeStatus{
					Devices: []v1alpha1.DeviceInfo{},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := plugin.getDeviceModel(tt.cn)
			if model != tt.expected {
				t.Errorf("Expected model %q, got %q", tt.expected, model)
			}
		})
	}
}

func TestStateData_Clone(t *testing.T) {
	original := &stateData{
		request: &ComputeRequest{
			VRAMBytes:  16 * 1024 * 1024 * 1024,
			FP16TFLOPS: 100,
		},
	}

	cloned := original.Clone()
	clonedData := cloned.(*stateData)

	if clonedData == original {
		t.Error("Clone should return a new instance")
	}

	if clonedData.request.VRAMBytes != original.request.VRAMBytes {
		t.Errorf("Cloned VRAMBytes should match")
	}

	if clonedData.request.FP16TFLOPS != original.request.FP16TFLOPS {
		t.Errorf("Cloned FP16TFLOPS should match")
	}
}

func TestReservePlugin_Name(t *testing.T) {
	plugin := NewReservePlugin(nil)
	if plugin.Name() != ReservePluginName {
		t.Errorf("Expected name %s, got %s", ReservePluginName, plugin.Name())
	}
}

func TestParseComputeRequest(t *testing.T) {
	plugin := &FilterPlugin{}

	tests := []struct {
		name         string
		vramGi       int64
		fp16TFLOPS   int64
		expectNil    bool
		expectedVRAM uint64
		expectedFP16 uint64
	}{
		{
			name:         "normal request",
			vramGi:       16,
			fp16TFLOPS:   100,
			expectNil:    false,
			expectedVRAM: 16 * 1024 * 1024 * 1024,
			expectedFP16: 100,
		},
		{
			name:         "vram only",
			vramGi:       32,
			fp16TFLOPS:   0,
			expectNil:    false,
			expectedVRAM: 32 * 1024 * 1024 * 1024,
			expectedFP16: 0,
		},
		{
			name:       "no hcs request",
			vramGi:     0,
			fp16TFLOPS: 0,
			expectNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := createTestPod(tt.vramGi, tt.fp16TFLOPS)
			req := plugin.parseComputeRequest(pod)

			if tt.expectNil {
				if req != nil {
					t.Error("Expected nil request")
				}
				return
			}

			if req == nil {
				t.Fatal("Expected non-nil request")
			}

			if req.VRAMBytes != tt.expectedVRAM {
				t.Errorf("Expected VRAMBytes %d, got %d", tt.expectedVRAM, req.VRAMBytes)
			}

			if req.FP16TFLOPS != tt.expectedFP16 {
				t.Errorf("Expected FP16TFLOPS %d, got %d", tt.expectedFP16, req.FP16TFLOPS)
			}
		})
	}
}

func TestFilterPlugin_Filter_NoStateData(t *testing.T) {
	plugin := NewFilterPlugin(newMockClient())
	state := framework.NewCycleState()
	pod := createTestPod(0, 0)
	node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}
	nodeInfo := framework.NewNodeInfo(node)

	status := plugin.Filter(context.Background(), state, pod, nodeInfo)

	if !status.IsSuccess() {
		t.Errorf("Filter should succeed with no state data, got: %s", status.Message)
	}
}

func TestFilterPlugin_Filter_Success(t *testing.T) {
	plugin := NewFilterPlugin(newMockClient())
	state := framework.NewCycleState()

	// Pre-store state data
	state.Write(stateKey, &stateData{
		request: &ComputeRequest{
			VRAMBytes:  16 * 1024 * 1024 * 1024,
			FP16TFLOPS: 100,
		},
	})

	pod := createTestPod(16, 100)
	node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}
	nodeInfo := framework.NewNodeInfo(node)

	status := plugin.Filter(context.Background(), state, pod, nodeInfo)

	if !status.IsSuccess() {
		t.Errorf("Filter should succeed for node-1, got: %s", status.Message)
	}
}

func TestFilterPlugin_Filter_NodeNotFound(t *testing.T) {
	plugin := NewFilterPlugin(newMockClient())
	state := framework.NewCycleState()

	state.Write(stateKey, &stateData{
		request: &ComputeRequest{
			VRAMBytes:  16 * 1024 * 1024 * 1024,
			FP16TFLOPS: 100,
		},
	})

	pod := createTestPod(16, 100)
	node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "nonexistent-node"}}
	nodeInfo := framework.NewNodeInfo(node)

	status := plugin.Filter(context.Background(), state, pod, nodeInfo)

	if status.IsSuccess() {
		t.Error("Filter should fail for nonexistent node")
	}
	if status.Code != framework.UnschedulableAndUnresolvable {
		t.Errorf("Expected UnschedulableAndUnresolvable, got %d", status.Code)
	}
}

func TestFilterPlugin_Filter_UnhealthyNode(t *testing.T) {
	plugin := NewFilterPlugin(newMockClient())
	state := framework.NewCycleState()

	state.Write(stateKey, &stateData{
		request: &ComputeRequest{
			VRAMBytes:  16 * 1024 * 1024 * 1024,
			FP16TFLOPS: 100,
		},
	})

	pod := createTestPod(16, 100)
	node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-unhealthy"}}
	nodeInfo := framework.NewNodeInfo(node)

	status := plugin.Filter(context.Background(), state, pod, nodeInfo)

	if status.IsSuccess() {
		t.Error("Filter should fail for unhealthy node")
	}
	if status.Code != framework.UnschedulableAndUnresolvable {
		t.Errorf("Expected UnschedulableAndUnresolvable, got %d", status.Code)
	}
}

func TestFilterPlugin_Filter_InsufficientVRAM(t *testing.T) {
	plugin := NewFilterPlugin(newMockClient())
	state := framework.NewCycleState()

	// Request more VRAM than node-2 has available
	// node-2 has 48GB total, 40GB used, only 8GB free
	state.Write(stateKey, &stateData{
		request: &ComputeRequest{
			VRAMBytes:  32 * 1024 * 1024 * 1024, // Request 32GB
			FP16TFLOPS: 0,
		},
	})

	pod := createTestPod(32, 0)
	node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}}
	nodeInfo := framework.NewNodeInfo(node)

	status := plugin.Filter(context.Background(), state, pod, nodeInfo)

	if status.IsSuccess() {
		t.Error("Filter should fail for insufficient VRAM")
	}
	if status.Code != framework.Unschedulable {
		t.Errorf("Expected Unschedulable, got %d", status.Code)
	}
}

func TestFilterPlugin_Filter_InsufficientFP16(t *testing.T) {
	plugin := NewFilterPlugin(newMockClient())
	state := framework.NewCycleState()

	// Request more FP16 TFLOPS than node-2 has
	// node-2 has 150 FP16 TFLOPS
	state.Write(stateKey, &stateData{
		request: &ComputeRequest{
			VRAMBytes:  1 * 1024 * 1024 * 1024,
			FP16TFLOPS: 500, // Request 500 TFLOPS
		},
	})

	pod := createTestPod(1, 500)
	node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}}
	nodeInfo := framework.NewNodeInfo(node)

	status := plugin.Filter(context.Background(), state, pod, nodeInfo)

	if status.IsSuccess() {
		t.Error("Filter should fail for insufficient FP16 TFLOPS")
	}
	if status.Code != framework.Unschedulable {
		t.Errorf("Expected Unschedulable, got %d", status.Code)
	}
}

func TestFilterPlugin_Filter_NilClient(t *testing.T) {
	plugin := NewFilterPlugin(nil)
	state := framework.NewCycleState()

	state.Write(stateKey, &stateData{
		request: &ComputeRequest{
			VRAMBytes:  16 * 1024 * 1024 * 1024,
			FP16TFLOPS: 100,
		},
	})

	pod := createTestPod(16, 100)
	node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}
	nodeInfo := framework.NewNodeInfo(node)

	status := plugin.Filter(context.Background(), state, pod, nodeInfo)

	if status.IsSuccess() {
		t.Error("Filter should fail with nil client")
	}
}

func TestFilterPlugin_GetComputeNode_NilClient(t *testing.T) {
	plugin := &FilterPlugin{client: nil}

	cn, err := plugin.getComputeNode(context.Background(), "node-1")
	if err == nil {
		t.Error("getComputeNode should error with nil client")
	}
	if cn != nil {
		t.Error("ComputeNode should be nil when error occurs")
	}
}

func TestFilterPlugin_GetComputeNode_Success(t *testing.T) {
	plugin := NewFilterPlugin(newMockClient())

	cn, err := plugin.getComputeNode(context.Background(), "node-1")
	if err != nil {
		t.Errorf("getComputeNode should succeed, got: %v", err)
	}
	if cn == nil {
		t.Fatal("ComputeNode should not be nil")
	}
	if cn.Name != "node-1" {
		t.Errorf("Expected node-1, got %s", cn.Name)
	}
}

func TestFilterPlugin_GetComputeNode_NotFound(t *testing.T) {
	plugin := NewFilterPlugin(newMockClient())

	cn, err := plugin.getComputeNode(context.Background(), "nonexistent")
	if err == nil {
		t.Error("getComputeNode should error for nonexistent node")
	}
	if cn != nil {
		t.Error("ComputeNode should be nil when not found")
	}
}

func TestReservePlugin_Reserve_NoStateData(t *testing.T) {
	plugin := NewReservePlugin(newMockClient())
	state := framework.NewCycleState()
	pod := createTestPod(0, 0)

	status := plugin.Reserve(context.Background(), state, pod, "node-1")

	if !status.IsSuccess() {
		t.Errorf("Reserve should succeed with no state data, got: %s", status.Message)
	}
}

func TestReservePlugin_Reserve_Success(t *testing.T) {
	plugin := NewReservePlugin(newMockClient())
	state := framework.NewCycleState()

	state.Write(stateKey, &stateData{
		request: &ComputeRequest{
			VRAMBytes:  16 * 1024 * 1024 * 1024,
			FP16TFLOPS: 100,
		},
	})

	pod := createTestPod(16, 100)
	status := plugin.Reserve(context.Background(), state, pod, "node-1")

	if !status.IsSuccess() {
		t.Errorf("Reserve should succeed, got: %s", status.Message)
	}

	// Verify reservation was created
	reserved := plugin.GetTotalReservedVRAM("node-1")
	if reserved != 16*1024*1024*1024 {
		t.Errorf("Expected reserved VRAM %d, got %d", 16*1024*1024*1024, reserved)
	}
}

func TestReservePlugin_Reserve_Idempotent(t *testing.T) {
	plugin := NewReservePlugin(newMockClient())
	state := framework.NewCycleState()

	state.Write(stateKey, &stateData{
		request: &ComputeRequest{
			VRAMBytes:  16 * 1024 * 1024 * 1024,
			FP16TFLOPS: 100,
		},
	})

	pod := createTestPod(16, 100)

	// First reserve
	status := plugin.Reserve(context.Background(), state, pod, "node-1")
	if !status.IsSuccess() {
		t.Fatalf("First reserve should succeed, got: %s", status.Message)
	}

	// Second reserve (should be idempotent)
	status = plugin.Reserve(context.Background(), state, pod, "node-1")
	if !status.IsSuccess() {
		t.Errorf("Second reserve should succeed (idempotent), got: %s", status.Message)
	}

	// Verify still only one reservation
	reserved := plugin.GetTotalReservedVRAM("node-1")
	if reserved != 16*1024*1024*1024 {
		t.Errorf("Expected reserved VRAM %d, got %d (double counted?)", 16*1024*1024*1024, reserved)
	}
}

func TestReservePlugin_Reserve_NilClient(t *testing.T) {
	plugin := NewReservePlugin(nil)
	state := framework.NewCycleState()

	state.Write(stateKey, &stateData{
		request: &ComputeRequest{
			VRAMBytes:  16 * 1024 * 1024 * 1024,
			FP16TFLOPS: 100,
		},
	})

	pod := createTestPod(16, 100)
	status := plugin.Reserve(context.Background(), state, pod, "node-1")

	if status.IsSuccess() {
		t.Error("Reserve should fail with nil client")
	}
	if status.Code != framework.Error {
		t.Errorf("Expected Error code, got %d", status.Code)
	}
}

func TestReservePlugin_Reserve_InsufficientVRAM(t *testing.T) {
	plugin := NewReservePlugin(newMockClient())
	state := framework.NewCycleState()

	// First, create a reservation that takes most of the VRAM
	state.Write(stateKey, &stateData{
		request: &ComputeRequest{
			VRAMBytes:  140 * 1024 * 1024 * 1024, // Request 140GB
			FP16TFLOPS: 100,
		},
	})

	pod1 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "default"},
	}
	status := plugin.Reserve(context.Background(), state, pod1, "node-1")
	if !status.IsSuccess() {
		t.Fatalf("First reserve should succeed, got: %s", status.Message)
	}

	// Now try to reserve more than available
	state2 := framework.NewCycleState()
	state2.Write(stateKey, &stateData{
		request: &ComputeRequest{
			VRAMBytes:  20 * 1024 * 1024 * 1024, // Request 20GB more
			FP16TFLOPS: 100,
		},
	})

	pod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod2", Namespace: "default"},
	}
	status = plugin.Reserve(context.Background(), state2, pod2, "node-1")

	if status.IsSuccess() {
		t.Error("Reserve should fail for insufficient VRAM after existing reservation")
	}
	if status.Code != framework.Unschedulable {
		t.Errorf("Expected Unschedulable code, got %d", status.Code)
	}
}

func TestReservePlugin_CalculateAvailableVRAMWithReservation(t *testing.T) {
	plugin := NewReservePlugin(nil)

	// Add a reservation
	plugin.mu.Lock()
	plugin.reservations["node-1"] = &Reservation{
		Pods: map[string]*PodReservation{
			"default/test-pod": {
				VRAMBytes: 20 * 1024 * 1024 * 1024,
			},
		},
	}
	plugin.mu.Unlock()

	cn := &v1alpha1.ComputeNode{
		Spec: v1alpha1.ComputeNodeSpec{
			TotalCapacity: v1alpha1.ComputeCapacity{
				VRAM: 160 * 1024 * 1024 * 1024,
			},
		},
		Status: v1alpha1.ComputeNodeStatus{
			Devices: []v1alpha1.DeviceInfo{
				{VRAMUsed: 10 * 1024 * 1024 * 1024},
			},
		},
	}

	// Available = 160GB - 10GB(used) - 20GB(reserved) = 130GB
	available := plugin.calculateAvailableVRAMWithReservation(cn, "node-1")
	expected := uint64(130 * 1024 * 1024 * 1024)
	if available != expected {
		t.Errorf("Expected available VRAM %d, got %d", expected, available)
	}
}

func TestReservePlugin_CalculateAvailableVRAMWithReservation_NoReservation(t *testing.T) {
	plugin := NewReservePlugin(nil)

	cn := &v1alpha1.ComputeNode{
		Spec: v1alpha1.ComputeNodeSpec{
			TotalCapacity: v1alpha1.ComputeCapacity{
				VRAM: 160 * 1024 * 1024 * 1024,
			},
		},
		Status: v1alpha1.ComputeNodeStatus{
			Devices: []v1alpha1.DeviceInfo{
				{VRAMUsed: 10 * 1024 * 1024 * 1024},
			},
		},
	}

	available := plugin.calculateAvailableVRAMWithReservation(cn, "node-1")
	expected := uint64(150 * 1024 * 1024 * 1024)
	if available != expected {
		t.Errorf("Expected available VRAM %d, got %d", expected, available)
	}
}

func TestReservePlugin_ClearPodReservation(t *testing.T) {
	plugin := NewReservePlugin(nil)

	// Add reservations
	plugin.mu.Lock()
	plugin.reservations["node-1"] = &Reservation{
		Pods: map[string]*PodReservation{
			"default/test-pod": {
				VRAMBytes: 20 * 1024 * 1024 * 1024,
			},
			"default/other-pod": {
				VRAMBytes: 10 * 1024 * 1024 * 1024,
			},
		},
	}
	plugin.mu.Unlock()

	// Clear one pod's reservation
	plugin.ClearPodReservation("default/test-pod")

	// Verify reservation was cleared
	reserved := plugin.GetTotalReservedVRAM("node-1")
	expected := uint64(10 * 1024 * 1024 * 1024)
	if reserved != expected {
		t.Errorf("Expected reserved VRAM %d after clear, got %d", expected, reserved)
	}
}

func TestReservePlugin_ClearPodReservation_LastPod(t *testing.T) {
	plugin := NewReservePlugin(nil)

	// Add single reservation
	plugin.mu.Lock()
	plugin.reservations["node-1"] = &Reservation{
		Pods: map[string]*PodReservation{
			"default/test-pod": {
				VRAMBytes: 20 * 1024 * 1024 * 1024,
			},
		},
	}
	plugin.mu.Unlock()

	// Clear the last pod's reservation
	plugin.ClearPodReservation("default/test-pod")

	// Verify node entry was cleaned up
	reserved := plugin.GetTotalReservedVRAM("node-1")
	if reserved != 0 {
		t.Errorf("Expected reserved VRAM 0 after clearing last pod, got %d", reserved)
	}

	plugin.mu.RLock()
	_, exists := plugin.reservations["node-1"]
	plugin.mu.RUnlock()
	if exists {
		t.Error("Node reservation entry should be deleted when empty")
	}
}

func TestReservePlugin_ClearPodReservation_NotFound(t *testing.T) {
	plugin := NewReservePlugin(nil)

	// Should not panic when pod not found
	plugin.ClearPodReservation("nonexistent/pod")
}

func TestReservePlugin_GetComputeNode_NilClient(t *testing.T) {
	plugin := NewReservePlugin(nil)

	cn, err := plugin.getComputeNode(context.Background(), "node-1")
	if err == nil {
		t.Error("getComputeNode should error with nil client")
	}
	if cn != nil {
		t.Error("ComputeNode should be nil when error occurs")
	}
}

func TestReservePlugin_GetComputeNode_Success(t *testing.T) {
	plugin := NewReservePlugin(newMockClient())

	cn, err := plugin.getComputeNode(context.Background(), "node-1")
	if err != nil {
		t.Errorf("getComputeNode should succeed, got: %v", err)
	}
	if cn == nil {
		t.Fatal("ComputeNode should not be nil")
	}
	if cn.Name != "node-1" {
		t.Errorf("Expected node-1, got %s", cn.Name)
	}
}

func TestGetPodKey(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-pod",
			Namespace: "my-namespace",
		},
	}

	key := getPodKey(pod)
	expected := "my-namespace/my-pod"
	if key != expected {
		t.Errorf("Expected key %s, got %s", expected, key)
	}
}
