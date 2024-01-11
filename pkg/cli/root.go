package cli

import (
	"github.com/acorn-io/cmd"
	"github.com/spf13/cobra"
)

type AssistantRuntime struct {
	cmd.DebugLogging
}

func (a *AssistantRuntime) PersistentPre(cmd *cobra.Command, args []string) error {
	return a.InitLogging()
}

func New() *cobra.Command {
	return cmd.Command(&AssistantRuntime{},
		&Controller{},
		&Server{},
		&Chat{})
}

func (a *AssistantRuntime) Run(cmd *cobra.Command, args []string) error {
	return nil
}
