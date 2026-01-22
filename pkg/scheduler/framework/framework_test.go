package framework

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewStatus(t *testing.T) {
	tests := []struct {
		name    string
		code    StatusCode
		message string
	}{
		{"success", Success, "ok"},
		{"unschedulable", Unschedulable, "not enough resources"},
		{"error", Error, "internal error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := NewStatus(tt.code, tt.message)
			if status.Code != tt.code {
				t.Errorf("Expected code %d, got %d", tt.code, status.Code)
			}
			if status.Message != tt.message {
				t.Errorf("Expected message '%s', got '%s'", tt.message, status.Message)
			}
		})
	}
}

func TestStatus_IsSuccess(t *testing.T) {
	tests := []struct {
		name     string
		status   *Status
		expected bool
	}{
		{"nil status", nil, true},
		{"success code", &Status{Code: Success}, true},
		{"unschedulable", &Status{Code: Unschedulable}, false},
		{"error", &Status{Code: Error}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.status.IsSuccess() != tt.expected {
				t.Errorf("IsSuccess() = %v, want %v", tt.status.IsSuccess(), tt.expected)
			}
		})
	}
}

func TestNewCycleState(t *testing.T) {
	state := NewCycleState()
	if state == nil {
		t.Fatal("NewCycleState should return non-nil")
	}

	if state.data == nil {
		t.Error("CycleState.data should be initialized")
	}
}

// testStateData 用于测试的状态数据
type testStateData struct {
	value string
}

func (d *testStateData) Clone() StateData {
	return &testStateData{value: d.value}
}

func TestCycleState_WriteRead(t *testing.T) {
	state := NewCycleState()
	data := &testStateData{value: "test-value"}

	// 写入
	state.Write("test-key", data)

	// 读取
	read, err := state.Read("test-key")
	if err != nil {
		t.Fatalf("Read() should not error: %v", err)
	}

	readData := read.(*testStateData)
	if readData.value != "test-value" {
		t.Errorf("Expected value 'test-value', got '%s'", readData.value)
	}
}

func TestCycleState_Read_NotFound(t *testing.T) {
	state := NewCycleState()

	_, err := state.Read("nonexistent")
	if err == nil {
		t.Error("Read() should error for nonexistent key")
	}

	notFoundErr, ok := err.(*NotFoundError)
	if !ok {
		t.Errorf("Expected NotFoundError, got %T", err)
	}

	if notFoundErr.Key != "nonexistent" {
		t.Errorf("Expected key 'nonexistent', got '%s'", notFoundErr.Key)
	}
}

func TestNotFoundError_Error(t *testing.T) {
	err := &NotFoundError{Key: "my-key"}
	expected := "state not found: my-key"

	if err.Error() != expected {
		t.Errorf("Error() = '%s', want '%s'", err.Error(), expected)
	}
}

func TestNewNodeInfo(t *testing.T) {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
	}

	nodeInfo := NewNodeInfo(node)
	if nodeInfo == nil {
		t.Fatal("NewNodeInfo should return non-nil")
	}

	if nodeInfo.Node() != node {
		t.Error("NodeInfo should contain the same node")
	}

	if nodeInfo.Node().Name != "test-node" {
		t.Errorf("Expected node name 'test-node', got '%s'", nodeInfo.Node().Name)
	}
}

func TestNewNodeInfo_Nil(t *testing.T) {
	nodeInfo := NewNodeInfo(nil)
	if nodeInfo == nil {
		t.Fatal("NewNodeInfo should return non-nil even with nil node")
	}

	if nodeInfo.Node() != nil {
		t.Error("Node() should return nil when created with nil")
	}
}

func TestStatusCodes(t *testing.T) {
	// 验证状态码的值
	if Success != 0 {
		t.Errorf("Success should be 0, got %d", Success)
	}
	if Unschedulable != 1 {
		t.Errorf("Unschedulable should be 1, got %d", Unschedulable)
	}
	if UnschedulableAndUnresolvable != 2 {
		t.Errorf("UnschedulableAndUnresolvable should be 2, got %d", UnschedulableAndUnresolvable)
	}
	if Error != 3 {
		t.Errorf("Error should be 3, got %d", Error)
	}
}

func TestCycleState_Overwrite(t *testing.T) {
	state := NewCycleState()

	// 写入初始值
	state.Write("key", &testStateData{value: "value1"})

	// 覆盖
	state.Write("key", &testStateData{value: "value2"})

	// 读取
	read, err := state.Read("key")
	if err != nil {
		t.Fatalf("Read() failed: %v", err)
	}

	if read.(*testStateData).value != "value2" {
		t.Errorf("Expected overwritten value 'value2', got '%s'", read.(*testStateData).value)
	}
}

func TestStateData_Clone(t *testing.T) {
	original := &testStateData{value: "original"}
	cloned := original.Clone()

	if cloned == original {
		t.Error("Clone should return a new instance")
	}

	clonedData := cloned.(*testStateData)
	if clonedData.value != "original" {
		t.Errorf("Cloned value should be 'original', got '%s'", clonedData.value)
	}

	// 修改 clone 不影响原始值
	clonedData.value = "modified"
	if original.value != "original" {
		t.Error("Modifying clone should not affect original")
	}
}
