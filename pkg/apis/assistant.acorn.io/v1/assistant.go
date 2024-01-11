package v1

import (
	"encoding/json"
	"strings"

	"github.com/acorn-io/aml/pkg/jsonschema"
	"github.com/acorn-io/baaah/pkg/conditions"
	"github.com/acorn-io/z"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	_ conditions.Conditions = (*Assistant)(nil)
)

const (
	ToolTypeFunction ToolType = "function"
)

type ToolType string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Assistant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AssistantSpec   `json:"spec,omitempty"`
	Status AssistantStatus `json:"status,omitempty"`
}

func (in *Assistant) GetDescription() string {
	return in.Spec.Description
}

func (in *Assistant) GetConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

const UserAnnotationPrefix = "user.ai.acorn.io/"

func AddMetadataToAnnotations(annotation map[string]string, metadata *map[string]any) error {
	for k, v := range z.Dereference(metadata) {
		s, err := json.Marshal(v)
		if err != nil {
			return err
		}
		annotation[UserAnnotationPrefix+k] = string(s)
	}
	return nil
}

func MetadataFromAnnotation(anno map[string]string) *map[string]any {
	result := map[string]any{}
	for k, v := range anno {
		if !strings.HasPrefix(k, UserAnnotationPrefix) {
			continue
		}
		k = strings.TrimPrefix(k, UserAnnotationPrefix)
		// Does this work?
		var a any
		err := json.Unmarshal([]byte(v), &a)
		if err != nil {
			continue
		}
		result[k] = a
	}
	return &result
}

type AssistantSpec struct {
	Name         string             `json:"name,omitempty"`
	Description  string             `json:"description,omitempty"`
	Instructions string             `json:"instructions,omitempty"`
	Model        string             `json:"model,omitempty"`
	Vision       bool               `json:"vision,omitempty"`
	Tools        []Tool             `json:"tools,omitempty"`
	Parameters   *jsonschema.Schema `json:"parameters"`
	MaxTokens    int                `json:"maxTokens,omitempty"`
	JSONResponse bool               `json:"jsonResponse,omitempty"`
	Cache        *bool              `json:"cache,omitempty"`
}

type FunctionDefinition struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Domain      string             `json:"domain,omitempty"`
	Parameters  *jsonschema.Schema `json:"parameters"`
}

type Tool struct {
	Type     ToolType           `json:"type"`
	Function FunctionDefinition `json:"function,omitempty"`
}

type AssistantStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AssistantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Assistant `json:"items"`
}
