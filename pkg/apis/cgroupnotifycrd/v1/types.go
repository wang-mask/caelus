package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tencent/caelus/pkg/util/times"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CgroupNotifyCrd is a specification for a CgroupNotifyCrd resource
type CgroupNotifyCrd struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CgroupNotifyCrdSpec   `json:"spec"`
	Status CgroupNotifyCrdStatus `json:"status"`
}

// CgroupNotifyCrdSpec is the spec for a CgroupNotifyCrd resource
type CgroupNotifyCrdSpec struct {
	MemoryCgroup MemoryNotifyConfig   `json:"memory_cgroup"`
	NodeSelector metav1.LabelSelector `json:"nodeSelector"`
}

// MemoryNotifyConfig describe memory cgroup notify
type MemoryNotifyConfig struct {
	Pressures []MemoryPressureNotifyConfig `json:"pressures"`
	Usages    []MemoryUsageNotifyConfig    `json:"usages"`
}

// MemoryPressureNotifyConfig describe memory.pressure_level notify data
type MemoryPressureNotifyConfig struct {
	Cgroups       []string `json:"cgroups"`
	PressureLevel string   `json:"pressure_level"`
	// assign time duration the pressure has kept
	Duration times.Duration `json:"duration"`
	// assign event number in the duration time
	Count int `json:"count"`
}

// MemoryUsageNotifyConfig describe memory.usage_in_bytes notify data
type MemoryUsageNotifyConfig struct {
	Cgroups []string `json:"cgroups"`
	// the distance between limit and threshold
	MarginMb int `json:"margin_mb"`
	// when to handle event after receiving event
	Duration times.Duration `json:"duration"`
}

// CgroupNotifyCrdStatus is the status for a CgroupNotifyCrd resource
type CgroupNotifyCrdStatus struct {
	AvailableReplicas int32 `json:"availableReplicas"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CgroupNotifyCrdList is a list of CgroupNotifyCrd resources
type CgroupNotifyCrdList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CgroupNotifyCrd `json:"items"`
}
