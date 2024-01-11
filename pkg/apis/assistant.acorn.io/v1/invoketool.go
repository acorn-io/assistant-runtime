package v1

import (
	"github.com/acorn-io/baaah/pkg/conditions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	_ conditions.Conditions = (*InvokeTool)(nil)
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type InvokeTool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InvokeToolSpec   `json:"spec,omitempty"`
	Status InvokeToolStatus `json:"status,omitempty"`
}

func (in *InvokeTool) GetConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

type InvokeToolSpec struct {
	ThreadName        string   `json:"threadName,omitempty"`
	ParentMessageName string   `json:"parentMessageName,omitempty"`
	ToolCall          ToolCall `json:"toolCall,omitempty"`
}

type InvokeToolStatus struct {
	Content    []ContentPart      `json:"content,omitempty"`
	InProgress bool               `json:"inProgress,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type InvokeToolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []InvokeTool `json:"items"`
}
