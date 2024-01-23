package invoketool

import (
	v1 "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1"
	"github.com/acorn-io/baaah/pkg/router"
	apierror "k8s.io/apimachinery/pkg/api/errors"
)

func Handle(req router.Request, resp router.Response) error {
	invoke := req.Object.(*v1.InvokeTool)

	var (
		assistant v1.Assistant
		thread    v1.Thread
	)

	if err := req.Get(&thread, req.Namespace, invoke.Spec.ThreadName); err != nil {
		return err
	}

	if err := req.Get(&assistant, req.Namespace, invoke.Spec.ToolCall.Function.Name); apierror.IsNotFound(err) {
		if len(invoke.Status.Content) == 0 || invoke.Generation != invoke.Status.Generation {
			if err := req.Get(&assistant, req.Namespace, thread.Spec.AssistantName); err != nil {
				return err
			}
			body, err := callFunc(req.Ctx, &assistant, invoke.Spec.ToolCall)
			if err != nil {
				return err
			}
			invoke.Status.Content = body.Content
		}
		invoke.Status.InProgress = false
	} else if err != nil {
		return err
	} else if msg, ok, err := callAssistant(req, resp, &thread, invoke.Spec.ToolCall); err != nil {
		return err
	} else if !ok {
		return nil
	} else {
		invoke.Status.Content = msg.Status.Message.Content
		invoke.Status.InProgress = msg.Status.InProgress
	}

	invoke.Status.Generation = invoke.Generation
	return nil
}
