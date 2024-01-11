package v1

import (
	"fmt"

	"github.com/acorn-io/baaah/pkg/conditions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	_ conditions.Conditions = (*Thread)(nil)
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Thread struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ThreadSpec   `json:"spec,omitempty"`
	Status ThreadStatus `json:"status,omitempty"`
}

func (in *Thread) GetDescription() string {
	return fmt.Sprintf("%s (created %s)", in.Name, in.CreationTimestamp)
}

func (in *Thread) GetConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

type ThreadSpec struct {
	ParentThreadName string `json:"parentThreadName,omitempty"`
	StartMessageName string `json:"startMessageName,omitempty"`
	AssistantName    string `json:"assistantName,omitempty"`
}

type ThreadStatus struct {
	Description string             `json:"description,omitempty"`
	Conditions  []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ThreadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Thread `json:"items"`
}
