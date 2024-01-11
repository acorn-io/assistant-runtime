package appspec

import (
	"context"
	"strings"

	"github.com/acorn-io/aml/pkg/jsonschema"
	v1 "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1"
	"github.com/acorn-io/assistant-runtime/pkg/vision"
	"github.com/acorn-io/baaah/pkg/router"
	acornv1 "github.com/acorn-io/runtime/pkg/apis/api.acorn.io/v1"
	acorninternalv1 "github.com/acorn-io/runtime/pkg/apis/internal.acorn.io/v1"
	"github.com/acorn-io/runtime/pkg/publicname"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	defaultAssistantFuncSchema = jsonschema.Schema{
		Property: jsonschema.Property{
			Type: "object",
		},
		Properties: map[string]jsonschema.Property{
			"input": {
				Type: "string",
			},
		},
		Required: []string{"input"},
	}
)

func getInputSchema(ctx context.Context, c kclient.Client, namespace string, appSpec *acorninternalv1.AppSpec, targetNamespace, name string) (*v1.FunctionDefinition, error) {
	assistant, ok := appSpec.Assistants[name]
	if ok {
		assistant := v1.Assistant{}
		if err := c.Get(ctx, router.Key(namespace, name), &assistant); apierrors.IsNotFound(err) {
			return nil, nil
		} else if err != nil {
			return nil, err
		}

		return &v1.FunctionDefinition{
			Name:        name,
			Description: assistant.Spec.Description,
			Parameters:  assistant.Spec.Parameters,
		}, nil
	}

	function, ok := appSpec.Functions[name]
	if ok {
		schema := function.InputSchema
		if schema == nil {
			schema = &defaultAssistantFuncSchema
		}
		return &v1.FunctionDefinition{
			Name:        name,
			Description: assistant.Description,
			Parameters:  schema,
			Domain:      targetNamespace,
		}, nil
	}

	return nil, nil
}

type Handler struct {
	AppName string
}

func (h *Handler) Handle(req router.Request, resp router.Response) error {
	appSpec := req.Object.(*acornv1.App)

	if h.AppName != "" {
		parent, _ := publicname.Split(h.AppName)
		if publicname.Get(appSpec) != parent {
			return nil
		}
	}

	for name, assistant := range appSpec.Status.AppSpec.Assistants {
		var (
			tools       []v1.Tool
			inputSchema = assistant.InputSchema
		)

		friendlyName := assistant.Name
		if friendlyName == "" {
			friendlyName = name
		}

		for _, dep := range assistant.Dependencies {
			if dep.TargetName == name {
				continue
			}
			funcDef, err := getInputSchema(req.Ctx, req.Client, req.Namespace, &appSpec.Status.AppSpec, appSpec.Status.Namespace, dep.TargetName)
			if err != nil {
				return err
			}
			if funcDef != nil {
				tools = append(tools, v1.Tool{
					Type:     v1.ToolTypeFunction,
					Function: *funcDef,
				})
			}
		}

		if assistant.Vision {
			tools = nil
			inputSchema = &vision.Schema
		}

		if inputSchema == nil {
			inputSchema = &defaultAssistantFuncSchema
		}

		resp.Objects(&v1.Assistant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: appSpec.Namespace,
			},
			Spec: v1.AssistantSpec{
				Name:         friendlyName,
				Description:  assistant.Description,
				Instructions: strings.Join(assistant.Prompts, "\n"),
				Vision:       assistant.Vision,
				Model:        assistant.Model,
				Parameters:   inputSchema,
				Tools:        tools,
				MaxTokens:    assistant.MaxTokens,
				JSONResponse: assistant.JSONResponse,
				Cache:        assistant.Cache,
			},
		})
	}

	return nil
}
