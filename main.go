package main

import (
	"github.com/acorn-io/assistant-runtime/pkg/cli"
	"github.com/acorn-io/cmd"
)

func main() {
	cmd.Main(cli.New())
}
