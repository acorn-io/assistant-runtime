package controller

import (
	"context"

	assistant_acorn_io "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io"
	"github.com/acorn-io/assistant-runtime/pkg/openai"
	"github.com/acorn-io/assistant-runtime/pkg/scheme"
	"github.com/acorn-io/baaah"
	"github.com/acorn-io/baaah/pkg/restconfig"
	"github.com/acorn-io/baaah/pkg/router"
	"github.com/acorn-io/baaah/pkg/runtime"
)

type Services struct {
	AppName      string
	OpenAIClient *openai.Client
	Router       *router.Router
	PreStart     func(ctx context.Context) error
}

func NewServices(opt Options) (*Services, error) {
	openAIClient, err := openai.NewClient()
	if err != nil {
		return nil, err
	}

	apiServerRESTConfig, err := restconfig.FromURLTokenAndScheme(opt.ApiUrl, opt.ApiToken, scheme.Scheme)
	if err != nil {
		return nil, err
	}

	r, err := baaah.NewRouter("assistant-runtime-controller", &baaah.Options{
		DefaultRESTConfig: scheme.DefaultConfig(),
		DefaultNamespace:  opt.Namespace,
		Scheme:            scheme.Scheme,
		APIGroupConfigs: map[string]runtime.Config{
			assistant_acorn_io.Group: {
				Rest:      apiServerRESTConfig,
				Namespace: opt.Namespace,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return &Services{
		AppName:      opt.AppName,
		OpenAIClient: openAIClient,
		Router:       r,
		PreStart: func(ctx context.Context) error {
			return restconfig.WaitFor(ctx, apiServerRESTConfig)
		},
	}, nil
}
