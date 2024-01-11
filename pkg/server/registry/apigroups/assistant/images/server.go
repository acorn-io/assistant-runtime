package images

import (
	"context"
	"net/http"

	v1 "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1"
	"github.com/acorn-io/mink/pkg/strategy"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Serve struct {
	strategy.DestroyAdapter

	Client kclient.Client
}

func (s *Serve) New() runtime.Object {
	return &v1.NoOptions{}
}

func (s *Serve) Connect(ctx context.Context, id string, options runtime.Object, r rest.Responder) (http.Handler, error) {
	ns, _ := request.NamespaceFrom(ctx)
	img := &v1.Image{}
	if err := s.Client.Get(ctx, kclient.ObjectKey{Namespace: ns, Name: id}, img); err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", img.Spec.ContentType)
		_, _ = rw.Write(img.Spec.Content)
	}), nil
}

func (s *Serve) NewConnectOptions() (runtime.Object, bool, string) {
	return &v1.NoOptions{}, false, ""
}

func (s *Serve) ConnectMethods() []string {
	return []string{http.MethodGet}
}
