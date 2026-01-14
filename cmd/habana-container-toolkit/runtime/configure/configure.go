package configure

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/HabanaAI/habana-container-runtime/internal/runtime"
	"github.com/urfave/cli/v3"
)

const (
	defaultExecutableDir = "/usr/bin"
	defaultRuntime       = "containerd"

	defaultContainerdConfigFilePath = "/etc/containerd/config.toml"
	defaultCrioConfigFilePath       = "/etc/crio/crio.conf"
	defaultDockerConfigFilePath     = "/etc/docker/daemon.json"
)

type command struct {
	logger *logrus.Logger
}

func NewCommand(logger *logrus.Logger) *cli.Command {
	c := command{
		logger: logger,
	}
	return c.build()
}

// config defines the options that can be set for the CLI through config files,
// environment variables, or command line config
type config struct {
	dryRun          bool
	runtime         string
	configFilePath  string
	remove          bool
	restartOnChange bool

	habanaRuntime struct {
		binariesDir string
	}

	// cdi-specific options
	cdi struct {
		enabled bool
	}
}

func (m command) build() *cli.Command {
	// Create a config struct to hold the parsed environment variables or command line flags
	config := config{}

	// Create the 'configure' command
	configure := cli.Command{
		Name:  "configure",
		Usage: "Add a runtime to the specified container engine",
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			return ctx, m.validateFlags(&config)
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return m.configureWrapper(&config)
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "dry-run",
				Usage:       "update the runtime configuration as required but don't write changes to disk",
				Destination: &config.dryRun,
			},
			&cli.StringFlag{
				Name:        "runtime",
				Usage:       "the target runtime engine; one of [containerd, crio, docker]",
				Value:       defaultRuntime,
				Destination: &config.runtime,
			},
			&cli.StringFlag{
				Name:        "config",
				Usage:       "path to the config file for the target runtime",
				Destination: &config.configFilePath,
			},
			&cli.BoolFlag{
				Name:        "enable-cdi",
				Usage:       "enable CDI support for the Habana runtime",
				Destination: &config.cdi.enabled,
			},
			&cli.StringFlag{
				Name:        "habana-runtime-binaries-dir",
				Usage:       "directory where the Habana runtime binaries are located",
				Value:       defaultExecutableDir,
				Destination: &config.habanaRuntime.binariesDir,
			},
			&cli.BoolFlag{
				Name:        "remove",
				Usage:       "remove the Habana runtime from the specified container engine",
				Value:       false,
				Destination: &config.remove,
			},
			&cli.BoolFlag{
				Name:        "restart-on-change",
				Usage:       "restart the container engine if the configuration was modified",
				Value:       false,
				Destination: &config.restartOnChange,
			},
		},
		Description: "The 'configure' command is used to add a Habana runtime to the specified container engine.",
	}

	return &configure
}

func (m command) validateFlags(config *config) error {
	if config.configFilePath == "" {
		switch config.runtime {
		case "containerd":
			config.configFilePath = defaultContainerdConfigFilePath
		case "crio":
			config.configFilePath = defaultCrioConfigFilePath
		case "docker":
			config.configFilePath = defaultDockerConfigFilePath
		}
	}

	return nil
}

func (m command) configureWrapper(config *config) error {
	cfg, err := runtime.NewRuntimeBuilder(
		config.runtime,
		m.logger,
		config.configFilePath,
	)
	if err != nil {
		return fmt.Errorf("unable to create runtime builder: %v", err)
	}

	if config.remove {
		err := m.removeRuntime(cfg)
		if err != nil {
			return fmt.Errorf("unable to remove runtime: %v", err)
		}
	} else {
		err := m.configureRuntime(cfg, config)
		if err != nil {
			return fmt.Errorf("unable to configure runtime: %v", err)
		}
	}

	if config.dryRun {
		m.logger.Info("Dry run mode enabled, not saving changes to disk")
		output, err := cfg.Serialize()
		if err != nil {
			return fmt.Errorf("unable to serialize configuration: %v", err)
		}
		m.logger.Infof("Dry run output:\n%s", string(output))
		return nil
	}

	isModified, err := cfg.IsModified()
	if err != nil {
		return fmt.Errorf("unable to determine if configuration was modified: %v", err)
	}

	if !isModified {
		m.logger.Infof("No changes to configuration")
		return nil
	}

	// Save the updated configuration to the specified file path
	m.logger.Infof("Saving configuration to %s", config.configFilePath)
	err = cfg.Save(config.configFilePath)
	if err != nil {
		return fmt.Errorf("unable to flush config: %v", err)
	}

	if config.restartOnChange {
		m.logger.Infof("Restarting %s due to configuration change", config.runtime)
		if err := cfg.RestartRuntime(); err != nil {
			return fmt.Errorf("unable to restart runtime: %v", err)
		}
	}

	return nil
}

func (m command) configureRuntime(builder runtime.RuntimeBuilder, config *config) error {
	err := builder.AddRuntime(
		config.habanaRuntime.binariesDir,
	)
	if err != nil {
		return fmt.Errorf("unable to update config: %v", err)
	}

	if config.cdi.enabled {
		if err := builder.EnableCDI(); err != nil {
			return fmt.Errorf("unable to enable CDI: %v", err)
		}
	}

	return nil
}

func (m command) removeRuntime(builder runtime.RuntimeBuilder) error {
	err := builder.RemoveRuntime()
	if err != nil {
		return fmt.Errorf("unable to update config: %v", err)
	}
	return nil
}

func runCommand(cmd string) error {
	name := strings.Split(cmd, " ")[0]
	args := strings.Split(cmd, " ")
	c := exec.Command(name, args[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
