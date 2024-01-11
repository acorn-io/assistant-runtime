package cli

import (
	"github.com/acorn-io/assistant-runtime/pkg/chat"
	"github.com/acorn-io/assistant-runtime/pkg/scheme"
	"github.com/acorn-io/baaah/pkg/restconfig"
	"github.com/acorn-io/cmd"
	"github.com/spf13/cobra"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Chat struct {
	cmd.DebugLogging
	chat.Options

	URL   string `usage:"URL of assistant runtime API" default:"http://localhost:8080"`
	Token string `usage:"Bearer token to talk to assistant runtime API"`
}

func (c *Chat) Run(cmd *cobra.Command, args []string) error {
	if err := c.DebugLogging.InitLogging(); err != nil {
		return err
	}

	restConfig, err := restconfig.FromURLTokenAndScheme(c.URL, c.Token, scheme.Scheme)
	if err != nil {
		return err
	}

	if err := restconfig.WaitFor(cmd.Context(), restConfig); err != nil {
		return err
	}

	client, err := kclient.NewWithWatch(restConfig, kclient.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		return err
	}

	return chat.Run(cmd.Context(), client, c.Options)
}
