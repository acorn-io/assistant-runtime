package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/acorn-io/assistant-runtime/pkg/openapi/generated"
	"github.com/acorn-io/assistant-runtime/pkg/scheme"
	"github.com/acorn-io/assistant-runtime/pkg/server/registry/apigroups/assistant"
	"github.com/acorn-io/assistant-runtime/pkg/server/services"
	"github.com/acorn-io/assistant-runtime/pkg/version"
	"github.com/acorn-io/mink/brent"
	mserver "github.com/acorn-io/mink/pkg/server"
	k8sversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/server/healthz"
)

func Run(ctx context.Context, config services.Config) error {
	services, err := services.New(config)
	if err != nil {
		return err
	}

	if err := startMinkServer(ctx, config, services); err != nil {
		return err
	}

	<-ctx.Done()
	return ctx.Err()
}

func startMinkServer(ctx context.Context, cfg services.Config, services *services.Services) error {
	apiGroups, err := mserver.BuildAPIGroups(services, assistant.APIGroup)
	if err != nil {
		return err
	}
	minkConfig := &mserver.Config{
		Name:              "Assistant Runtime",
		Version:           version.Get().String(),
		Authenticator:     services.Authn,
		HTTPListenPort:    cfg.HTTPListenPort,
		HTTPSListenPort:   cfg.HTTPSListenPort,
		OpenAPIConfig:     generated.GetOpenAPIDefinitions,
		Scheme:            scheme.Scheme,
		APIGroups:         apiGroups,
		ReadinessCheckers: []healthz.HealthChecker{services.DB},
	}

	if cfg.AuditLogPolicyFile != "" && cfg.AuditLogPath != "" {
		minkConfig.AuditConfig = mserver.NewAuditOptions(cfg.AuditLogPolicyFile, cfg.AuditLogPath)
	}

	brentHandler, brentStartHook, err := brent.Handler(ctx, &brent.Config{
		RESTConfig: services.RESTConfig,
		MinkConfig: minkConfig,
	})
	if err != nil {
		return err
	}

	minkConfig.Middleware = []func(http.Handler) http.Handler{
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				if req.URL.Path == "/version" {
					_ = json.NewEncoder(rw).Encode(k8sversion.Info{
						GitVersion: version.Get().String(),
						GitCommit:  version.Get().Commit,
					})
				} else if strings.HasPrefix(req.URL.Path, "/v1") {
					brentHandler.ServeHTTP(rw, req)
				} else {
					next.ServeHTTP(rw, req)
				}
			})
		},
	}
	minkConfig.PostStartFunc = brentStartHook

	minkServer, err := mserver.New(minkConfig)
	if err != nil {
		return err
	}

	return minkServer.Run(ctx)
}
