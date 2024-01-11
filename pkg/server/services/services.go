package services

import (
	_ "embed"
	"fmt"

	"github.com/acorn-io/assistant-runtime/pkg/scheme"
	"github.com/acorn-io/baaah/pkg/randomtoken"
	"github.com/acorn-io/baaah/pkg/ratelimit"
	"github.com/acorn-io/baaah/pkg/restconfig"
	"github.com/acorn-io/mink/pkg/db"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/client-go/rest"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Config struct {
	HTTPListenPort     int    `usage:"HTTP port to listen on" default:"8080"`
	HTTPSListenPort    int    `usage:"HTTPS port to listen on"`
	AdminToken         string `usage:"Token for admin access, will be generated if not passed"`
	AuditLogPath       string `usage:"Location of where to store audit logs"`
	AuditLogPolicyFile string `usage:"Location of audit log policy file"`
	DSN                string `usage:"Database dsn in driver://connection_string format" default:"sqlite://file:assistant.db?_journal=WAL&cache=shared&_busy_timeout=30000"`
}

func New(config Config) (_ *Services, err error) {
	if config.AdminToken == "" {
		config.AdminToken, err = randomtoken.Generate()
		if err != nil {
			return nil, err
		}
	}

	downstreamConfig := restconfig.SetScheme(&rest.Config{
		Host: fmt.Sprintf("http://127.0.0.1:%d", config.HTTPListenPort),
		//BearerToken: config.AdminToken,
		RateLimiter: ratelimit.None,
	}, scheme.Scheme)
	downstreamClient, err := kclient.NewWithWatch(downstreamConfig, kclient.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		return nil, err
	}

	dbClient, err := db.NewFactory(scheme.Scheme, config.DSN)
	if err != nil {
		return nil, err
	}

	services := &Services{
		Client:     downstreamClient,
		RESTConfig: downstreamConfig,
		DB:         dbClient,
	}

	return services, nil
}

type Services struct {
	Client     kclient.Client
	RESTConfig *rest.Config
	DB         *db.Factory
	Authn      authenticator.Request
}
