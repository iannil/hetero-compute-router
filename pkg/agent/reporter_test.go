package agent

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/zrs-products/hetero-compute-router/pkg/api/v1alpha1"
	"github.com/zrs-products/hetero-compute-router/pkg/collectors"
	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// mockClient 用于测试的模拟 K8s 客户端
type mockClient struct {
	computeNodes map[string]*v1alpha1.ComputeNode
	createErr    error
	updateErr    error
	getErr       error
	deleteErr    error
}

func newMockK8sClient() *mockClient {
	return &mockClient{
		computeNodes: make(map[string]*v1alpha1.ComputeNode),
	}
}

func (c *mockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if c.getErr != nil {
		return c.getErr
	}

	cn, ok := c.computeNodes[key.Name]
	if !ok {
		return errors.NewNotFound(schema.GroupResource{Group: "hcs.io", Resource: "computenodes"}, key.Name)
	}

	*obj.(*v1alpha1.ComputeNode) = *cn
	return nil
}

func (c *mockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if c.createErr != nil {
		return c.createErr
	}
	cn := obj.(*v1alpha1.ComputeNode)
	c.computeNodes[cn.Name] = cn.DeepCopy()
	return nil
}

func (c *mockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if c.updateErr != nil {
		return c.updateErr
	}
	cn := obj.(*v1alpha1.ComputeNode)
	if _, ok := c.computeNodes[cn.Name]; !ok {
		return errors.NewNotFound(schema.GroupResource{}, cn.Name)
	}
	c.computeNodes[cn.Name] = cn.DeepCopy()
	return nil
}

func (c *mockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if c.deleteErr != nil {
		return c.deleteErr
	}
	cn := obj.(*v1alpha1.ComputeNode)
	delete(c.computeNodes, cn.Name)
	return nil
}

func (c *mockClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}

func (c *mockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}

func (c *mockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
}

type mockStatusWriter struct {
	client *mockClient
}

func (c *mockClient) Status() client.StatusWriter {
	return &mockStatusWriter{client: c}
}

func (c *mockClient) Scheme() *runtime.Scheme {
	return nil
}

func (c *mockClient) RESTMapper() meta.RESTMapper {
	return nil
}

func (c *mockClient) SubResource(subResource string) client.SubResourceClient {
	return nil
}

func (c *mockClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (c *mockClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return false, nil
}

func (s *mockStatusWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	return nil
}

func (s *mockStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return s.client.Update(ctx, obj)
}

func (s *mockStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return nil
}

func TestNewReporter(t *testing.T) {
	c := newMockK8sClient()
	reporter := NewReporter(c)

	if reporter == nil {
		t.Fatal("NewReporter should return non-nil")
	}
}

func TestReporter_Report_Create(t *testing.T) {
	c := newMockK8sClient()
	reporter := NewReporter(c)

	hwType := &detectors.HardwareType{
		Vendor:          "nvidia",
		DriverAvailable: true,
		DriverVersion:   "535.104.05",
	}

	devices := []*detectors.Device{
		{
			ID:          "gpu-0",
			Model:       "NVIDIA A100",
			VRAMTotal:   80 * 1024 * 1024 * 1024,
			VRAMUsed:    10 * 1024 * 1024 * 1024,
			HealthScore: 95.0,
			PCIEBusID:   "0000:01:00.0",
		},
	}

	metrics := &collectors.Metrics{
		Fingerprint: &collectors.FingerprintMetrics{
			VRAMTotal: 80 * 1024 * 1024 * 1024,
			ComputeCap: collectors.ComputeMetrics{
				FP16TFLOPS: 312,
				FP32TFLOPS: 19,
			},
		},
		Health: &collectors.HealthMetrics{
			Score: 95.0,
		},
	}

	ctx := context.Background()
	err := reporter.Report(ctx, "test-node", hwType, devices, metrics)
	if err != nil {
		t.Fatalf("Report() failed: %v", err)
	}

	// 验证创建的 ComputeNode
	if _, ok := c.computeNodes["test-node"]; !ok {
		t.Error("ComputeNode should be created")
	}
}

func TestReporter_Report_Update(t *testing.T) {
	c := newMockK8sClient()
	reporter := NewReporter(c)

	// 预先创建一个 ComputeNode
	c.computeNodes["test-node"] = &v1alpha1.ComputeNode{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
		Spec: v1alpha1.ComputeNodeSpec{
			NodeName: "test-node",
		},
	}

	hwType := &detectors.HardwareType{
		Vendor:          "nvidia",
		DriverAvailable: true,
		DriverVersion:   "535.104.05",
	}

	devices := []*detectors.Device{
		{
			ID:          "gpu-0",
			Model:       "NVIDIA A100",
			VRAMTotal:   80 * 1024 * 1024 * 1024,
			HealthScore: 100.0,
		},
	}

	metrics := &collectors.Metrics{
		Fingerprint: &collectors.FingerprintMetrics{
			VRAMTotal: 80 * 1024 * 1024 * 1024,
		},
		Health: &collectors.HealthMetrics{
			Score: 100.0,
		},
	}

	ctx := context.Background()
	err := reporter.Report(ctx, "test-node", hwType, devices, metrics)
	if err != nil {
		t.Fatalf("Report() failed on update: %v", err)
	}
}

func TestReporter_Report_CreateError(t *testing.T) {
	c := newMockK8sClient()
	c.createErr = fmt.Errorf("create error")
	reporter := NewReporter(c)

	hwType := &detectors.HardwareType{
		Vendor:          "nvidia",
		DriverAvailable: true,
	}
	devices := []*detectors.Device{}
	metrics := &collectors.Metrics{
		Fingerprint: &collectors.FingerprintMetrics{},
		Health:      &collectors.HealthMetrics{Score: 80.0},
	}

	ctx := context.Background()
	err := reporter.Report(ctx, "test-node", hwType, devices, metrics)
	if err == nil {
		t.Error("Report() should fail on create error")
	}
}

func TestReporter_Report_GetError(t *testing.T) {
	c := newMockK8sClient()
	c.getErr = fmt.Errorf("get error")
	reporter := NewReporter(c)

	hwType := &detectors.HardwareType{
		Vendor:          "nvidia",
		DriverAvailable: true,
	}
	devices := []*detectors.Device{}
	metrics := &collectors.Metrics{
		Fingerprint: &collectors.FingerprintMetrics{},
		Health:      &collectors.HealthMetrics{Score: 80.0},
	}

	ctx := context.Background()
	err := reporter.Report(ctx, "test-node", hwType, devices, metrics)
	if err == nil {
		t.Error("Report() should fail on get error")
	}
}

func TestReporter_Delete(t *testing.T) {
	c := newMockK8sClient()
	c.computeNodes["test-node"] = &v1alpha1.ComputeNode{}
	reporter := NewReporter(c)

	ctx := context.Background()
	err := reporter.Delete(ctx, "test-node")
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	if _, ok := c.computeNodes["test-node"]; ok {
		t.Error("ComputeNode should be deleted")
	}
}

func TestReporter_Delete_NotFound(t *testing.T) {
	c := newMockK8sClient()
	reporter := NewReporter(c)

	ctx := context.Background()
	err := reporter.Delete(ctx, "nonexistent")
	if err != nil {
		t.Error("Delete() should not error for nonexistent node")
	}
}

func TestReporter_Delete_Error(t *testing.T) {
	c := newMockK8sClient()
	c.deleteErr = fmt.Errorf("delete error")
	reporter := NewReporter(c)

	ctx := context.Background()
	err := reporter.Delete(ctx, "test-node")
	if err == nil {
		t.Error("Delete() should fail on error")
	}
}

func TestReporter_BuildComputeNode_DriverNotAvailable(t *testing.T) {
	c := newMockK8sClient()
	reporter := NewReporter(c)

	hwType := &detectors.HardwareType{
		Vendor:          "nvidia",
		DriverAvailable: false,
	}

	devices := []*detectors.Device{}

	metrics := &collectors.Metrics{
		Fingerprint: &collectors.FingerprintMetrics{},
		Health:      &collectors.HealthMetrics{Score: 80.0},
	}

	cn := reporter.buildComputeNode("test-node", hwType, devices, metrics)

	// 检查 DriverAvailable 条件
	var foundDriver bool
	for _, cond := range cn.Status.Conditions {
		if cond.Type == v1alpha1.ComputeNodeConditionDriverAvailable {
			foundDriver = true
			if cond.Status != "False" {
				t.Error("Driver condition should be False when driver not available")
			}
		}
	}
	if !foundDriver {
		t.Error("Should have driver condition")
	}
}

func TestReporter_BuildComputeNode_NoDevices(t *testing.T) {
	c := newMockK8sClient()
	reporter := NewReporter(c)

	hwType := &detectors.HardwareType{
		Vendor:          "nvidia",
		DriverAvailable: true,
	}

	devices := []*detectors.Device{}

	metrics := &collectors.Metrics{
		Fingerprint: &collectors.FingerprintMetrics{},
		Health:      &collectors.HealthMetrics{Score: 80.0},
	}

	cn := reporter.buildComputeNode("test-node", hwType, devices, metrics)

	// 检查 DevicesReady 条件
	var foundDevices bool
	for _, cond := range cn.Status.Conditions {
		if cond.Type == v1alpha1.ComputeNodeConditionDevicesReady {
			foundDevices = true
			if cond.Status != "False" {
				t.Error("Devices condition should be False when no devices")
			}
		}
	}
	if !foundDevices {
		t.Error("Should have devices condition")
	}
}

func TestReporter_BuildComputeNode_LowHealth(t *testing.T) {
	c := newMockK8sClient()
	reporter := NewReporter(c)

	hwType := &detectors.HardwareType{
		Vendor:          "nvidia",
		DriverAvailable: true,
	}

	devices := []*detectors.Device{{ID: "gpu-0"}}

	metrics := &collectors.Metrics{
		Fingerprint: &collectors.FingerprintMetrics{},
		Health:      &collectors.HealthMetrics{Score: 20.0}, // Critical
	}

	cn := reporter.buildComputeNode("test-node", hwType, devices, metrics)

	if cn.Status.Phase != v1alpha1.ComputeNodePhaseUnhealthy {
		t.Errorf("Phase should be Unhealthy for low health score, got %s", cn.Status.Phase)
	}
}

func TestReporter_BuildComputeNode_WarningHealth(t *testing.T) {
	c := newMockK8sClient()
	reporter := NewReporter(c)

	hwType := &detectors.HardwareType{
		Vendor:          "nvidia",
		DriverAvailable: true,
	}

	devices := []*detectors.Device{{ID: "gpu-0"}}

	metrics := &collectors.Metrics{
		Fingerprint: &collectors.FingerprintMetrics{},
		Health:      &collectors.HealthMetrics{Score: 45.0}, // Warning
	}

	cn := reporter.buildComputeNode("test-node", hwType, devices, metrics)

	if cn.Status.Phase != v1alpha1.ComputeNodePhaseReady {
		t.Errorf("Phase should be Ready for warning health score, got %s", cn.Status.Phase)
	}
}

func TestReporter_BuildComputeNode_NilHealth(t *testing.T) {
	c := newMockK8sClient()
	reporter := NewReporter(c)

	hwType := &detectors.HardwareType{
		Vendor:          "nvidia",
		DriverAvailable: true,
	}

	devices := []*detectors.Device{{ID: "gpu-0"}}

	metrics := &collectors.Metrics{
		Fingerprint: &collectors.FingerprintMetrics{},
		Health:      nil,
	}

	cn := reporter.buildComputeNode("test-node", hwType, devices, metrics)

	if cn.Status.Phase != v1alpha1.ComputeNodePhaseUnhealthy {
		t.Errorf("Phase should be Unhealthy when health is nil, got %s", cn.Status.Phase)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig("my-node")

	if config == nil {
		t.Fatal("DefaultConfig should return non-nil")
	}

	if config.NodeName != "my-node" {
		t.Errorf("Expected NodeName 'my-node', got '%s'", config.NodeName)
	}

	if config.CollectInterval == 0 {
		t.Error("CollectInterval should have default value")
	}

	if config.ReportInterval == 0 {
		t.Error("ReportInterval should have default value")
	}

	if config.UseMock {
		t.Error("UseMock should be false by default")
	}
}
