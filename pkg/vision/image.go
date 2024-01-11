package vision

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	v1 "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1"
	"github.com/acorn-io/assistant-runtime/pkg/hash"
	"github.com/acorn-io/baaah/pkg/apply"
	"github.com/acorn-io/baaah/pkg/router"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	urlBase = os.Getenv("IMAGE_URL_BASE")
)

func ToVisionMessage(ctx context.Context, c kclient.Client, namespace string, message v1.MessageBody) (v1.MessageBody, error) {
	if len(message.Content) != 1 || !strings.HasPrefix(message.Content[0].Text, "{") {
		return message, nil
	}

	var (
		input   inputMessage
		content = message.Content[0]
	)
	if err := json.Unmarshal([]byte(content.Text), &input); err != nil {
		return message, nil
	}

	content.Text = input.Text

	if input.URL != "" {
		b64, contentType, err := Base64FromStored(ctx, c, namespace, input.URL)
		if err != nil {
			return message, err
		}
		if b64 == "" {
			content.Image = &v1.ChatMessageImageURL{
				URL: input.URL,
			}
		} else {
			input.Base64 = b64
			input.ContentType = contentType
		}
	}

	if input.Base64 != "" && input.ContentType != "" {
		content.Image = &v1.ChatMessageImageURL{
			Base64:      input.Base64,
			ContentType: input.ContentType,
		}
	}

	message.Content = []v1.ContentPart{
		content,
	}

	return message, nil
}

func Base64FromStored(ctx context.Context, c kclient.Client, namespace string, url string) (string, string, error) {
	if !strings.HasPrefix(url, urlBase) {
		return "", "", nil
	}
	parts := strings.Split(url, "/")
	if len(parts) < 3 {
		return "", "", nil
	}
	urlNamespace, name := parts[len(parts)-4], parts[len(parts)-2]
	if urlNamespace != namespace {
		return "", "", nil
	}

	var image v1.Image
	if err := c.Get(ctx, router.Key(urlNamespace, name), &image); apierrors.IsNotFound(err) {
		return "", "", nil
	} else if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(image.Spec.Content), image.Spec.ContentType, nil
}

func ImageToURL(ctx context.Context, k8s kclient.Client, namespace string, vision bool, message v1.ChatMessageImageURL) (string, error) {
	if message.URL != "" {
		return message.URL, nil
	}

	if vision {
		return fmt.Sprintf("data:%s;base64,%s", message.ContentType, message.Base64), nil
	}

	data, err := base64.StdEncoding.DecodeString(message.Base64)
	if err != nil {
		return "", err
	}

	id := "i" + hash.Encode(message)[:12]
	err = apply.New(k8s).Ensure(ctx, &v1.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name:      id,
			Namespace: namespace,
		},
		Spec: v1.ImageSpec{
			ContentType: message.ContentType,
			Content:     data,
		},
	})
	return fmt.Sprintf("%s/apis/assistant.acorn.io/v1/namespaces/%s/images/%s/serve", urlBase, namespace, id), err
}
