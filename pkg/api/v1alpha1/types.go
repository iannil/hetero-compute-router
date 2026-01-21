package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComputeNode is the Schema for the computenodes API
type ComputeNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComputeNodeSpec   `json:"spec,omitempty"`
	Status ComputeNodeStatus `json:"status,omitempty"`
}

// ComputeNodeSpec defines the desired state of ComputeNode
type ComputeNodeSpec struct {
	// NodeName is the Kubernetes node name
	NodeName string `json:"nodeName"`

	// Vendor is the hardware vendor (nvidia, huawei, hygon, cambricon)
	Vendor string `json:"vendor"`

	// TotalCapacity is the total compute capacity
	TotalCapacity ComputeCapacity `json:"totalCapacity"`
}

// ComputeCapacity represents compute resource capacity
type ComputeCapacity struct {
	// VRAM in bytes
	VRAM uint64 `json:"vram"`

	// FP16 TFLOPS
	FP16TFLOPS uint64 `json:"fp16Tflops,omitempty"`

	// FP32 TFLOPS
	FP32TFLOPS uint64 `json:"fp32Tflops,omitempty"`
}

// ComputeNodeStatus defines the observed state of ComputeNode
type ComputeNodeStatus struct {
	// Phase is the current phase of the compute node
	Phase ComputeNodePhase `json:"phase"`

	// Devices is the list of compute devices
	Devices []DeviceInfo `json:"devices,omitempty"`

	// Conditions represent the latest available observations
	Conditions []ComputeNodeCondition `json:"conditions,omitempty"`
}

// ComputeNodePhase represents the phase of a compute node
type ComputeNodePhase string

const (
	// ComputeNodePhaseInitializing means the node is being initialized
	ComputeNodePhaseInitializing ComputeNodePhase = "Initializing"
	// ComputeNodePhaseReady means the node is ready to accept workloads
	ComputeNodePhaseReady ComputeNodePhase = "Ready"
	// ComputeNodePhaseUnhealthy means the node has issues
	ComputeNodePhaseUnhealthy ComputeNodePhase = "Unhealthy"
	// ComputeNodePhaseTerminating means the node is being terminated
	ComputeNodePhaseTerminating ComputeNodePhase = "Terminating"
)

// DeviceInfo represents a single compute device
type DeviceInfo struct {
	// ID is the device identifier
	ID string `json:"id"`

	// Model is the device model name
	Model string `json:"model"`

	// VRAMTotal is the total VRAM in bytes
	VRAMTotal uint64 `json:"vramTotal"`

	// VRAMUsed is the used VRAM in bytes
	VRAMUsed uint64 `json:"vramUsed"`

	// HealthScore is the health score (0-100)
	HealthScore float64 `json:"healthScore"`

	// PCIEBusID is the PCI bus ID
	PCIEBusID string `json:"pcieBusID,omitempty"`

	// InterconnectType is the interconnect type (NVLink, HCCS, PCIe)
	InterconnectType string `json:"interconnectType,omitempty"`
}

// ComputeNodeCondition describes the state of a compute node at a certain point
type ComputeNodeCondition struct {
	// Type of condition
	Type ComputeNodeConditionType `json:"type"`

	// Status of the condition
	Status corev1.ConditionStatus `json:"status"`

	// LastTransitionTime is the last time the condition transitioned
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// Reason is the reason for the condition's last transition
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message indicating details
	Message string `json:"message,omitempty"`
}

// ComputeNodeConditionType represents a condition type
type ComputeNodeConditionType string

const (
	// ComputeNodeConditionDriverAvailable indicates if the driver is available
	ComputeNodeConditionDriverAvailable ComputeNodeConditionType = "DriverAvailable"
	// ComputeNodeConditionDevicesReady indicates if devices are ready
	ComputeNodeConditionDevicesReady ComputeNodeConditionType = "DevicesReady"
	// ComputeNodeConditionHealthy indicates if the node is healthy
	ComputeNodeConditionHealthy ComputeNodeConditionType = "Healthy"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComputeNodeList contains a list of ComputeNode
type ComputeNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeNode `json:"items"`
}
