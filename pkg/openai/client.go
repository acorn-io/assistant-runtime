package openai

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	v1 "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1"
	"github.com/acorn-io/assistant-runtime/pkg/hash"
	"github.com/acorn-io/assistant-runtime/pkg/vision"
	"github.com/acorn-io/baaah/pkg/router"
	"github.com/acorn-io/z"
	"github.com/sashabaranov/go-openai"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultVisionModel = openai.GPT4VisionPreview
	DefaultModel       = openai.GPT4TurboPreview
	DefaultMaxTokens   = 1024
)

var (
	key = os.Getenv("OPENAI_API_KEY")
	url = os.Getenv("xOPENAI_URL")
)

type Client struct {
	c *openai.Client
}

func NewClient() (*Client, error) {
	if url == "" {
		if key == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY env var is not set")
		}
		return &Client{
			c: openai.NewClient(key),
		}, nil
	}

	cfg := openai.DefaultConfig(key)
	cfg.BaseURL = url
	return &Client{
		c: openai.NewClientWithConfig(cfg),
	}, nil
}

func (c *Client) cacheKey(request openai.ChatCompletionRequest) string {
	return hash.Encode(request)
}

func (c *Client) fromCache(ctx context.Context, k8s kclient.Client, namespace string, messageRequest CompletionRequest, request openai.ChatCompletionRequest) (result []openai.ChatCompletionStreamResponse, _ bool, _ error) {
	if !z.Dereference(messageRequest.Cache) {
		return nil, false, nil
	}

	var cache v1.Cache
	if err := k8s.Get(ctx, router.Key(namespace, c.cacheKey(request)), &cache); apierrors.IsNotFound(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	gz, err := gzip.NewReader(bytes.NewReader(cache.Content))
	if err != nil {
		return nil, false, err
	}
	return result, true, json.NewDecoder(gz).Decode(&result)
}

func toToolCall(call v1.ToolCall) openai.ToolCall {
	return openai.ToolCall{
		Index: call.Index,
		ID:    call.ID,
		Type:  openai.ToolType(call.Type),
		Function: openai.FunctionCall{
			Name:      call.Function.Name,
			Arguments: call.Function.Arguments,
		},
	}
}

func toMessages(ctx context.Context, k8s kclient.Client, namespace string, request CompletionRequest) (result []openai.ChatCompletionMessage, err error) {
	for _, message := range request.Messages {
		if request.Vision {
			message, err = vision.ToVisionMessage(ctx, k8s, namespace, message)
			if err != nil {
				return nil, err
			}
		}

		chatMessage := openai.ChatCompletionMessage{
			Role: string(message.Role),
		}

		if message.ToolCall != nil {
			chatMessage.ToolCallID = message.ToolCall.ID
		}

		for _, content := range message.Content {
			if content.ToolCall != nil {
				chatMessage.ToolCalls = append(chatMessage.ToolCalls, toToolCall(*content.ToolCall))
			}
			if content.Image != nil {
				url, err := vision.ImageToURL(ctx, k8s, namespace, request.Vision, *content.Image)
				if err != nil {
					return nil, err
				}
				if request.Vision {
					chatMessage.MultiContent = append(chatMessage.MultiContent, openai.ChatMessagePart{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL: url,
						},
					})
				} else {
					chatMessage.MultiContent = append(chatMessage.MultiContent, openai.ChatMessagePart{
						Type: openai.ChatMessagePartTypeText,
						Text: fmt.Sprintf("Image URL %s", url),
					})
				}
			}
			if content.Text != "" {
				chatMessage.MultiContent = append(chatMessage.MultiContent, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: content.Text,
				})
			}
		}

		if len(chatMessage.MultiContent) == 1 && chatMessage.MultiContent[0].Type == openai.ChatMessagePartTypeText {
			if chatMessage.MultiContent[0].Text == "." {
				continue
			}
			chatMessage.Content = chatMessage.MultiContent[0].Text
			chatMessage.MultiContent = nil
		}

		result = append(result, chatMessage)
	}
	return
}

type CompletionRequest struct {
	Model        string
	Vision       bool
	Tools        []v1.Tool
	Messages     []v1.MessageBody
	MaxToken     int
	JSONResponse bool
	Cache        *bool
}

func (c *Client) Call(ctx context.Context, k8s kclient.Client, namespace string, messageRequest CompletionRequest, status chan<- v1.MessageBody) (*v1.MessageBody, error) {
	msgs, err := toMessages(ctx, k8s, namespace, messageRequest)
	if err != nil {
		return nil, err
	}

	request := openai.ChatCompletionRequest{
		Model:     messageRequest.Model,
		Messages:  msgs,
		MaxTokens: messageRequest.MaxToken,
	}

	if messageRequest.JSONResponse {
		request.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	if request.Model == "" {
		if messageRequest.Vision {
			request.Model = DefaultVisionModel
		} else {
			request.Model = DefaultModel
		}
	}

	if request.MaxTokens == 0 {
		request.MaxTokens = DefaultMaxTokens
	}

	if !messageRequest.Vision {
		for _, tool := range messageRequest.Tools {
			request.Tools = append(request.Tools, openai.Tool{
				Type: openai.ToolType(tool.Type),
				Function: openai.FunctionDefinition{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				},
			})
		}
	}

	request.Seed = z.Pointer(hash.Seed(request))
	response, ok, err := c.fromCache(ctx, k8s, namespace, messageRequest, request)
	if err != nil {
		return nil, err
	} else if !ok {
		response, err = c.call(ctx, k8s, namespace, request, status)
		if err != nil {
			return nil, err
		}
	}

	result := v1.MessageBody{}
	for _, response := range response {
		result = appendMessage(result, response)
	}

	return &result, nil
}

func appendMessage(msg v1.MessageBody, response openai.ChatCompletionStreamResponse) v1.MessageBody {
	if len(response.Choices) == 0 {
		return msg
	}

	delta := response.Choices[0].Delta
	msg.Role = v1.RoleType(override(string(msg.Role), delta.Role))

	for _, tool := range delta.ToolCalls {
		if tool.Index == nil {
			continue
		}
		idx := *tool.Index
		for len(msg.Content)-1 < idx {
			msg.Content = append(msg.Content, v1.ContentPart{
				ToolCall: &v1.ToolCall{
					Index: z.Pointer(len(msg.Content)),
				},
			})
		}

		tc := msg.Content[idx]
		tc.ToolCall.ID = override(tc.ToolCall.ID, tool.ID)
		tc.ToolCall.Type = v1.ToolType(override(string(tc.ToolCall.Type), string(tool.Type)))
		tc.ToolCall.Function.Name += tool.Function.Name
		tc.ToolCall.Function.Arguments += tool.Function.Arguments

		msg.Content[idx] = tc
	}

	if delta.Content != "" {
		found := false
		for i, content := range msg.Content {
			if content.ToolCall != nil || content.Image != nil {
				continue
			}
			msg.Content[i] = v1.ContentPart{
				Text: msg.Content[i].Text + delta.Content,
			}
			found = true
			break
		}
		if !found {
			msg.Content = append(msg.Content, v1.ContentPart{
				Text: delta.Content,
			})
		}
	}

	return msg
}

func override(left, right string) string {
	if right != "" {
		return right
	}
	return left
}

func (c *Client) store(ctx context.Context, k8s kclient.Client, key, namespace string, responses []openai.ChatCompletionStreamResponse) error {
	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	err := json.NewEncoder(gz).Encode(responses)
	if err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return k8s.Create(ctx, &v1.Cache{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key,
			Namespace: namespace,
		},
		Content: buf.Bytes(),
	})
}

func (c *Client) call(ctx context.Context, k8s kclient.Client, namespace string, request openai.ChatCompletionRequest, partial chan<- v1.MessageBody) (responses []openai.ChatCompletionStreamResponse, _ error) {
	cacheKey := c.cacheKey(request)
	request.Stream = true

	slog.Debug("calling openai", "message", request.Messages)
	stream, err := c.c.CreateChatCompletionStream(ctx, request)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			return responses, c.store(ctx, k8s, cacheKey, namespace, responses)
		} else if err != nil {
			return nil, err
		}
		if len(response.Choices) > 0 {
			slog.Debug("stream", "content", response.Choices[0].Delta.Content)
			if partial != nil {
				partial <- v1.MessageBody{
					Role:    v1.RoleType(response.Choices[0].Delta.Role),
					Content: v1.Text(response.Choices[0].Delta.Content),
				}
			}
		}
		responses = append(responses, response)
	}
}
