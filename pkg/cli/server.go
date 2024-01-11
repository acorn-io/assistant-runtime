package cli

import (
	"github.com/acorn-io/assistant-runtime/pkg/server"
	"github.com/acorn-io/assistant-runtime/pkg/server/services"
	"github.com/spf13/cobra"
)

type Server struct {
	services.Config
}

func (s *Server) Run(cmd *cobra.Command, args []string) error {
	return server.Run(cmd.Context(), s.Config)
}
