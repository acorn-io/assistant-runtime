package scheme

import (
	"log"

	"github.com/acorn-io/baaah/pkg/restconfig"
	"k8s.io/client-go/rest"
)

func DefaultConfig() *rest.Config {
	cfg, err := restconfig.New(Scheme)
	if err != nil {
		log.Fatalf("failed to build client config: %v", err)
	}
	return cfg
}
