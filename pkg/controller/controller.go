package controller

import (
	"context"
	"fmt"

	"github.com/acorn-io/baaah/pkg/router"
	// Enabled logrus logging in baaah
	_ "github.com/acorn-io/baaah/pkg/logrus"
)

type Options struct {
	ApiUrl    string `json:"apiUrl,omitempty" default:"http://localhost:8080"`
	ApiToken  string `json:"apiToken,omitempty"`
	Namespace string `usage:"Namespace to watch" default:"acorn"`
	AppName   string `usage:"App to create assistants for"`
}

type Controller struct {
	router   *router.Router
	services *Services
}

func New(ctx context.Context, opt Options) (*Controller, error) {
	services, err := NewServices(opt)
	if err != nil {
		return nil, err
	}

	err = routes(services.Router, services)
	if err != nil {
		return nil, err
	}

	return &Controller{
		router:   services.Router,
		services: services,
	}, nil
}

func (c *Controller) Start(ctx context.Context) error {
	if err := c.services.PreStart(ctx); err != nil {
		return err
	}
	if err := c.router.Start(ctx); err != nil {
		return fmt.Errorf("failed to start router: %w", err)
	}
	select {}
}
