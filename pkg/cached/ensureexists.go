package cached

import (
	"context"

	"github.com/acorn-io/baaah/pkg/router"
	"github.com/acorn-io/baaah/pkg/uncached"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func EnsureExists(ctx context.Context, c kclient.Client, namespace, name string, obj kclient.Object) (bool, error) {
	if name == "" {
		return false, nil
	}
	if err := c.Get(ctx, router.Key(namespace, name), obj); apierrors.IsNotFound(err) {
		err := c.Get(ctx, router.Key(namespace, name), uncached.Get(obj))
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return true, err
	} else if err != nil {
		return false, err
	}
	return true, nil
}
