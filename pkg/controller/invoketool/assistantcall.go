package invoketool

import (
	"context"
	"strings"

	v1 "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1"
	"github.com/acorn-io/baaah/pkg/name"
	"github.com/acorn-io/baaah/pkg/router"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func callAssistant(req router.Request, resp router.Response, threadParent *v1.Thread, call v1.ToolCall) (*v1.Message, bool, error) {
	callID := strings.ToLower(strings.ReplaceAll(call.ID, "_", "-"))
	threadName := name.SafeConcatName("t", callID)
	msgName := name.SafeConcatName("m", callID)
	assistantName := call.Function.Name

	req.Object.(*v1.InvokeTool).Status.AssistantMessageName = msgName

	resp.Objects(&v1.Thread{
		ObjectMeta: metav1.ObjectMeta{
			Name:      threadName,
			Namespace: req.Namespace,
		},
		Spec: v1.ThreadSpec{
			StartMessageName: msgName,
			ParentThreadName: threadParent.Name,
			AssistantName:    assistantName,
		},
	}, &v1.Message{
		ObjectMeta: metav1.ObjectMeta{
			Name:      msgName,
			Namespace: req.Namespace,
		},
		Spec: v1.MessageSpec{
			Input: v1.MessageInput{
				Content: v1.Text(call.Function.Arguments),
			},
		},
	})

	return getResponseMessage(req.Ctx, req.Client, req.Namespace, msgName)
}

func getResponseMessage(ctx context.Context, c kclient.Client, namespace, next string) (*v1.Message, bool, error) {
	for {
		var msg v1.Message

		if err := c.Get(ctx, router.Key(namespace, next), &msg); apierror.IsNotFound(err) {
			return nil, false, nil
		} else if err != nil {
			return nil, false, err
		}

		if msg.Status.NextMessageName != "" {
			next = msg.Status.NextMessageName
			continue
		} else if msg.Status.Message.Role == v1.RoleTypeAssistant && !msg.Status.Message.IsToolCall() {
			return &msg, true, nil
		}

		break
	}

	return nil, false, nil
}
