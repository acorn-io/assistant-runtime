package controller

import (
	v1 "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1"
	"github.com/acorn-io/assistant-runtime/pkg/controller/appspec"
	"github.com/acorn-io/assistant-runtime/pkg/controller/invoketool"
	"github.com/acorn-io/assistant-runtime/pkg/controller/message"
	"github.com/acorn-io/assistant-runtime/pkg/controller/thread"
	"github.com/acorn-io/baaah/pkg/apply"
	"github.com/acorn-io/baaah/pkg/conditions"
	"github.com/acorn-io/baaah/pkg/router"
	acornv1 "github.com/acorn-io/runtime/pkg/apis/api.acorn.io/v1"
)

func routes(router *router.Router, services *Services) error {
	messageHandler := message.NewGenerateHandler(services.OpenAIClient)

	root := router.Middleware(conditions.ErrorMiddleware())
	root.Type(&acornv1.App{}).Handler(&appspec.Handler{AppName: services.AppName})
	root.Type(&v1.Message{}).HandlerFunc(message.Initialize)

	withThread := root.Middleware(thread.IsSet)
	withThread.Type(&v1.Message{}).HandlerFunc(message.InvokeTools)
	withThread.Type(&v1.Message{}).HandlerFunc(messageHandler.CreateAssistantMessage)
	withThread.Type(&v1.Message{}).HandlerFunc(messageHandler.CompleteAssistant)

	root.Type(&v1.InvokeTool{}).HandlerFunc(invoketool.Handle)

	root.Type(&v1.InvokeTool{}).HandlerFunc(gc)
	root.Type(&v1.Assistant{}).HandlerFunc(gc)
	root.Type(&v1.Message{}).HandlerFunc(gc)
	root.Type(&v1.Thread{}).HandlerFunc(gc)

	return nil
}

func gc(req router.Request, resp router.Response) error {
	return apply.New(req.Client).PurgeOrphan(req.Ctx, req.Object)
}
