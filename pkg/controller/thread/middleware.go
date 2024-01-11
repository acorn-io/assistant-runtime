package thread

import (
	v1 "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1"
	"github.com/acorn-io/baaah/pkg/router"
)

func IsSet(next router.Handler) router.Handler {
	return router.HandlerFunc(func(req router.Request, resp router.Response) error {
		if m, ok := req.Object.(*v1.Message); ok && m.Status.ThreadName != "" {
			return next.Handle(req, resp)
		}
		return nil
	})
}
