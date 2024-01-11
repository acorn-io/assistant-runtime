package generic

import (
	"github.com/acorn-io/assistant-runtime/pkg/scheme"
	"github.com/acorn-io/mink/pkg/db"
	"github.com/acorn-io/mink/pkg/stores"
	"k8s.io/apiserver/pkg/registry/rest"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func NewStore(db *db.Factory, obj kclient.Object) (rest.Storage, rest.Storage, error) {
	storage, err := db.NewDBStrategy(obj)
	if err != nil {
		return nil, nil, err
	}

	return stores.NewComplete(scheme.Scheme, storage), stores.NewStatus(scheme.Scheme, storage), err
}
