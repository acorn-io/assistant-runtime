package v1

import (
	"fmt"
	"strings"

	"github.com/acorn-io/baaah/pkg/conditions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	_ conditions.Conditions = (*Message)(nil)
)

// Chat message role defined by the OpenAI API.
const (
	RoleTypeUser      = RoleType("user")
	RoleTypeAssistant = RoleType("assistant")
	RoleTypeTool      = RoleType("tool")
)

type RoleType string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Message struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MessageSpec   `json:"spec,omitempty"`
	Status MessageStatus `json:"status,omitempty"`
}

func (in *Message) GetDescription() string {
	content := in.Status.Message.String()
	if len(content) > 50 {
		return content + "..."
	}
	return content
}

func (in *Message) GetConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

type MessageBody struct {
	Role     RoleType      `json:"role,omitempty"`
	Content  []ContentPart `json:"content,omitempty" column:"name=Message,jsonpath=.spec.content"`
	ToolCall *ToolCall     `json:"toolCall,omitempty"`
}

func (in MessageBody) IsToolCall() bool {
	for _, content := range in.Content {
		if content.ToolCall != nil {
			return true
		}
	}
	return false
}

func Text(text string) []ContentPart {
	return []ContentPart{
		{
			Text: text,
		},
	}
}

func (in MessageBody) String() string {
	if !in.HasContent() {
		return ""
	}
	buf := strings.Builder{}
	if in.Role == RoleTypeUser {
		buf.WriteString("input -> ")
	} else if in.Role == RoleTypeTool && in.ToolCall != nil {
		buf.WriteString(fmt.Sprintf("tool return %s -> ", in.ToolCall.Function.Name))
	}
	for i, content := range in.Content {
		if i > 0 {
			buf.WriteString("\n -> ")
		}
		buf.WriteString(content.Text)
		if content.ToolCall != nil {
			buf.WriteString(fmt.Sprintf("tool call %s -> %s", content.ToolCall.Function.Name, content.ToolCall.Function.Arguments))
		}
		if content.Image != nil {
			buf.WriteString("image: ")
			if content.Image.URL != "" {
				buf.WriteString(content.Image.URL)
			}
			if len(content.Image.Base64) > 50 {
				buf.WriteString(content.Image.Base64[:50] + "...")
			} else {
				buf.WriteString(content.Image.Base64)
			}
		}
	}
	return buf.String()
}

type ContentPart struct {
	Text     string               `json:"text,omitempty"`
	ToolCall *ToolCall            `json:"toolCall,omitempty"`
	Image    *ChatMessageImageURL `json:"image,omitempty"`
}

func (in ContentPart) HasContent() bool {
	return in.Text != "" || in.ToolCall != nil || in.Image != nil
}

func (in MessageBody) HasContent() bool {
	return len(in.Content) > 0 && in.Content[0].HasContent()
}

type ImageURLDetail string

const (
	ImageURLDetailHigh ImageURLDetail = "high"
	ImageURLDetailLow  ImageURLDetail = "low"
	ImageURLDetailAuto ImageURLDetail = "auto"
)

type ChatMessageImageURL struct {
	Base64      string         `json:"base64,omitempty"`
	ContentType string         `json:"contentType,omitempty"`
	URL         string         `json:"url,omitempty"`
	Detail      ImageURLDetail `json:"detail,omitempty"`
}

type ToolCall struct {
	Index    *int         `json:"index,omitempty"`
	ID       string       `json:"id,omitempty"`
	Type     ToolType     `json:"type,omitempty"`
	Function FunctionCall `json:"function,omitempty"`
}

type FunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type MessageInput struct {
	Completion bool          `json:"completion,omitempty"`
	Content    []ContentPart `json:"content,omitempty"`
	ToolCall   *ToolCall     `json:"toolCall,omitempty"`
	InProgress bool          `json:"inProgress,omitempty"`
}

func (in MessageInput) Valid() error {
	if in.Completion && len(in.Content) > 0 {
		return fmt.Errorf("either spec.completion or spec.content should be set, but not both")
	}
	return nil
}

type MessageSpec struct {
	Input             MessageInput `json:"input,omitempty"`
	ParentMessageName string       `json:"parentMessageName,omitempty"`
	FileNames         []string     `json:"fileNames,omitempty"`
	More              bool         `json:"more,omitempty"`
}

type MessageStatus struct {
	Message         MessageBody        `json:"message,omitempty"`
	InProgress      bool               `json:"inProgress,omitempty"`
	RunAfter        *metav1.Time       `json:"runAfter,omitempty"`
	ThreadName      string             `json:"threadName,omitempty"`
	NextMessageName string             `json:"nextMessageName,omitempty"`
	InvokeToolNames []string           `json:"invokeToolNames,omitempty"`
	Conditions      []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MessageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Message `json:"items"`
}
