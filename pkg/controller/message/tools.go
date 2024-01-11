package message

import (
	"strings"

	v1 "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1"
	"github.com/acorn-io/baaah/pkg/name"
	"github.com/acorn-io/baaah/pkg/router"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func IsToolCall(msg *v1.Message) bool {
	return msg.Status.Message.Role == v1.RoleTypeAssistant &&
		!msg.Status.InProgress
}

func InvokeTools(req router.Request, resp router.Response) error {
	var (
		msg = req.Object.(*v1.Message)
	)

	if !IsToolCall(msg) {
		return nil
	}

	var (
		invokeToolNames []string
		thread          v1.Thread
	)

	if err := req.Get(&thread, req.Namespace, msg.Status.ThreadName); err != nil {
		return err
	}

	for _, content := range msg.Status.Message.Content {
		call := content.ToolCall
		if call == nil {
			continue
		}
		callID := strings.ToLower(strings.ReplaceAll(call.ID, "_", "-"))
		toolName := name.SafeConcatName(msg.Name, callID)
		invokeToolNames = append(invokeToolNames, toolName)

		resp.Objects(&v1.InvokeTool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      toolName,
				Namespace: msg.Namespace,
			},
			Spec: v1.InvokeToolSpec{
				ThreadName:        msg.Status.ThreadName,
				ParentMessageName: msg.Name,
				ToolCall:          *call,
			},
		})
	}

	var (
		lastMessage = msg.Name
	)

	for i, toolName := range invokeToolNames {
		var invoke v1.InvokeTool
		if err := req.Get(&invoke, msg.Namespace, toolName); apierror.IsNotFound(err) {
			// Ignore not found, it should be found later
			return nil
		} else if err != nil {
			return err
		}

		if len(invoke.Status.Content) == 0 {
			continue
		}

		toolMessage := v1.Message{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name.SafeConcatName(msg.Name, toolName),
				Namespace: msg.Namespace,
			},
			Spec: v1.MessageSpec{
				Input: v1.MessageInput{
					Content:    invoke.Status.Content,
					ToolCall:   &invoke.Spec.ToolCall,
					InProgress: invoke.Status.InProgress,
				},
				ParentMessageName: lastMessage,
				More:              i != len(invokeToolNames)-1,
			},
		}
		lastMessage = toolMessage.Name
		resp.Objects(&toolMessage)
	}

	return nil
}
