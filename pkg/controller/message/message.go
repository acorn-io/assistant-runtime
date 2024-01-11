package message

import (
	"context"

	v1 "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1"
	"github.com/acorn-io/baaah/pkg/conditions"
	"github.com/acorn-io/baaah/pkg/router"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func setThreadName(ctx context.Context, c kclient.Client, msg *v1.Message) error {
	if msg.Spec.ParentMessageName == "" {
		msg.Status.ThreadName = ""
		var threads v1.ThreadList
		err := c.List(ctx, &threads, &kclient.ListOptions{
			Namespace: msg.Namespace,
		})
		if err != nil {
			return err
		}
		for _, thread := range threads.Items {
			if thread.Spec.StartMessageName == msg.Name {
				msg.Status.ThreadName = thread.Name
				break
			}
		}

		return nil
	}

	var parent v1.Message
	if err := c.Get(ctx, router.Key(msg.Namespace, msg.Spec.ParentMessageName), &parent); err != nil {
		return err
	}
	if parent.Status.NextMessageName != "" && parent.Status.NextMessageName != msg.Name {
		return conditions.NewErrTerminalf("parent message [%s] is already a parent to [%s]", parent.Name, parent.Status.NextMessageName)
	}
	if parent.Status.NextMessageName == "" {
		parent.Status.NextMessageName = msg.Name
		if err := c.Status().Update(ctx, &parent); err != nil {
			return err
		}
	}

	msg.Status.ThreadName = parent.Status.ThreadName
	return nil
}

func Initialize(req router.Request, resp router.Response) error {
	msg := req.Object.(*v1.Message)

	if msg.Status.ThreadName != "" {
		var t v1.Thread
		if err := req.Get(&t, msg.Namespace, msg.Status.ThreadName); apierror.IsNotFound(err) {
			return req.Client.Delete(req.Ctx, msg)
		} else if err != nil {
			return err
		}
	}

	if err := setThreadName(req.Ctx, req.Client, msg); err != nil {
		return err
	}

	if err := msg.Spec.Input.Valid(); err != nil {
		return conditions.NewErrTerminal(err)
	}

	if msg.Spec.Input.InProgress {
		msg.Status.InProgress = msg.Spec.Input.InProgress
	}

	if msg.Spec.Input.Completion {
		msg.Status.Message.Role = v1.RoleTypeAssistant
		return nil
	}

	msg.Status.Message.Content = msg.Spec.Input.Content
	msg.Status.Message.ToolCall = msg.Spec.Input.ToolCall
	if msg.Status.Message.ToolCall == nil {
		msg.Status.Message.Role = v1.RoleTypeUser
	} else {
		msg.Status.Message.Role = v1.RoleTypeTool
	}
	return nil
}
