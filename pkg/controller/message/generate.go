package message

import (
	"context"
	"sync"
	"time"

	v1 "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1"
	openai2 "github.com/acorn-io/assistant-runtime/pkg/openai"
	"github.com/acorn-io/baaah/pkg/conditions"
	"github.com/acorn-io/baaah/pkg/name"
	"github.com/acorn-io/baaah/pkg/router"
	"github.com/sashabaranov/go-openai"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type CompleteClient interface {
	Call(ctx context.Context, k8s kclient.Client, namespace string, messageRequest openai2.CompletionRequest, status chan<- v1.MessageBody) (*v1.MessageBody, error)
}

func NewGenerateHandler(c CompleteClient) *Handler {
	return &Handler{
		oaiClient: c,
	}
}

type Handler struct {
	oaiClient CompleteClient
}

func (h *Handler) CompleteAssistant(req router.Request, resp router.Response) error {
	var (
		thread    v1.Thread
		assistant v1.Assistant
		msg       = req.Object.(*v1.Message)
		parent    = msg.DeepCopy()
	)

	if !msg.Spec.Input.Completion {
		return nil
	}

	msg.Status.InProgress = true

	if err := req.Get(&thread, msg.Namespace, msg.Status.ThreadName); err != nil {
		return err
	}

	if err := req.Get(&assistant, msg.Namespace, thread.Spec.AssistantName); err != nil {
		return err
	}

	request := openai2.CompletionRequest{
		Model:        assistant.Spec.Model,
		Tools:        assistant.Spec.Tools,
		Vision:       assistant.Spec.Vision,
		MaxToken:     assistant.Spec.MaxTokens,
		JSONResponse: assistant.Spec.JSONResponse,
		Cache:        assistant.Spec.Cache,
	}

	var msgs []v1.Message
	for parent.Spec.ParentMessageName != "" {
		if err := req.Get(parent, parent.Namespace, parent.Spec.ParentMessageName); err != nil {
			return err
		}
		msgs = append(msgs, *parent.DeepCopy())
	}

	if assistant.Spec.Instructions != "" {
		request.Messages = append(request.Messages, v1.MessageBody{
			Role:    openai.ChatMessageRoleSystem,
			Content: v1.Text(assistant.Spec.Instructions),
		})
	}

	for i := len(msgs) - 1; i >= 0; i-- {
		if !msgs[i].Status.Message.HasContent() || msgs[i].Status.InProgress {
			// Not ready
			return nil
		}
		request.Messages = append(request.Messages, msgs[i].Status.Message)
	}

	if err := h.complete(req.Ctx, req.Client, msg, request); err != nil {
		return err
	}

	msg.Status.InProgress = false
	return nil
}

func (h *Handler) CreateAssistantMessage(req router.Request, resp router.Response) error {
	var (
		msg = req.Object.(*v1.Message)
	)

	if msg.Spec.More {
		return nil
	}

	if msg.Status.Message.Role != v1.RoleTypeUser && msg.Status.Message.Role != v1.RoleTypeTool {
		return nil
	}

	response := v1.Message{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.SafeHashConcatName(msg.Name, "resp"),
			Namespace: req.Namespace,
		},
		Spec: v1.MessageSpec{
			Input: v1.MessageInput{
				Completion: true,
			},
			ParentMessageName: msg.Name,
		},
	}

	msg.Status.NextMessageName = response.Name
	resp.Objects(&response)
	return nil
}

func (h *Handler) runProgress(ctx context.Context, c kclient.Client, msg *v1.Message, progress chan v1.MessageBody) {
	var (
		current v1.MessageBody
	)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if current.HasContent() {
				msg.Status.Message = current
				_ = c.Status().Update(ctx, msg)
			}
		case msg, ok := <-progress:
			if len(msg.Content) > 0 && len(current.Content) > 0 && msg.Content[0].Text != "" &&
				msg.Content[0].Text != current.Content[0].Text {
				content := msg.Content[0]
				content.Text = current.Content[0].Text + content.Text
				msg.Content[0] = content
			}
			current = msg
			if !ok {
				break loop
			}
		}
	}

}

func (h *Handler) progress(ctx context.Context, k8s kclient.Client, msg *v1.Message) (chan<- v1.MessageBody, func()) {
	ctx, cancel := context.WithCancel(ctx)
	progress := make(chan v1.MessageBody, 2)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		h.runProgress(ctx, k8s, msg, progress)
		close(progress)
		wg.Done()
	}()

	return progress, func() {
		cancel()
		wg.Wait()
	}
}

func (h *Handler) complete(ctx context.Context, c kclient.Client, message *v1.Message, request openai2.CompletionRequest) error {
	progress, cancel := h.progress(ctx, c, message)
	defer cancel()

	result, err := h.oaiClient.Call(ctx, c, message.Namespace, request, progress)
	if err != nil {
		return conditions.NewErrTerminal(err)
	}

	cancel()

	message.Status.Message = *result
	return nil
}
