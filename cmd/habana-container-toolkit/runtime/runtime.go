package runtime

import (
	"github.com/HabanaAI/habana-container-runtime/cmd/habana-container-toolkit/runtime/configure"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

type runtimeCommand struct {
	logger *logrus.Logger
}

// NewCommand constructs a runtime command with the specified logger
func NewCommand(logger *logrus.Logger) *cli.Command {
	c := runtimeCommand{
		logger: logger,
	}
	return c.build()
}

func (m runtimeCommand) build() *cli.Command {
	runtime := cli.Command{
		Name:  "runtime",
		Usage: "A collection of runtime-related utilities for the Intel Gaudi Container Toolkit",
		Commands: []*cli.Command{
			configure.NewCommand(m.logger),
		},
	}

	return &runtime
}
