package invoketool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	v1 "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1"
	"github.com/acorn-io/baaah/pkg/conditions"
)

func callFunc(ctx context.Context, assistant *v1.Assistant, call v1.ToolCall) (body v1.MessageBody, _ error) {
	url := fmt.Sprintf("http://%s", call.Function.Name)
	for _, tool := range assistant.Spec.Tools {
		if tool.Function.Name == call.Function.Name && tool.Function.Domain != "" {
			url = fmt.Sprintf("http://%s.%s", call.Function.Name, tool.Function.Domain)
		}
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(call.Function.Arguments)))
	if err != nil {
		return body, err
	}
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return body, err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return body, err
	}
	if err = resp.Body.Close(); err != nil {
		return body, err
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "json") {
		if err := json.Unmarshal(data, &body); err != nil {
			return body, err
		}

		if !body.HasContent() {
			return body, conditions.NewErrTerminalf("function result did not generate a valid content fields or parts field: %s", data)
		}

		return body, nil
	}
	body.Content = []v1.ContentPart{
		{
			Text: string(data),
		},
	}
	if !body.HasContent() {
		return body, conditions.NewErrTerminalf("function returned an empty string")
	}
	return body, nil
}
