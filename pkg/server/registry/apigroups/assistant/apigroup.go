package assistant

import (
	v1 "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1"
	"github.com/acorn-io/assistant-runtime/pkg/scheme"
	"github.com/acorn-io/assistant-runtime/pkg/server/registry/apigroups/assistant/images"
	"github.com/acorn-io/assistant-runtime/pkg/server/registry/generic"
	"github.com/acorn-io/assistant-runtime/pkg/server/services"
	"github.com/acorn-io/baaah/pkg/typed"
	"github.com/acorn-io/mink/pkg/apigroup"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func Stores(services *services.Services) (map[string]rest.Storage, error) {
	result := map[string]rest.Storage{}

	var generics = map[string]kclient.Object{
		"assistants":  &v1.Assistant{},
		"caches":      &v1.Cache{},
		"invoketools": &v1.InvokeTool{},
		"messages":    &v1.Message{},
		"threads":     &v1.Thread{},
		"images":      &v1.Image{},
	}

	for _, name := range typed.SortedKeys(generics) {
		store, statusStore, err := generic.NewStore(services.DB, generics[name])
		if err != nil {
			return nil, err
		}

		result[name] = store
		result[name+"/status"] = statusStore
	}

	result["images/serve"] = &images.Serve{
		Client: services.Client,
	}

	return result, nil
}

func APIGroup(services *services.Services) (*genericapiserver.APIGroupInfo, error) {
	stores, err := Stores(services)
	if err != nil {
		return nil, err
	}
	return apigroup.ForStores(scheme.AddToScheme, stores, v1.SchemeGroupVersion)
}
