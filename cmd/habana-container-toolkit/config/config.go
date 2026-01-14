package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/HabanaAI/habana-container-runtime/internal/config"
	"github.com/HabanaAI/habana-container-runtime/internal/utils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

type args struct {
	dryRun         bool
	sets           []string
	configFilePath string
}

type command struct {
	logger *logrus.Logger
}

// NewCommand constructs a runtime command with the specified logger
func NewCommand(logger *logrus.Logger) *cli.Command {
	c := command{
		logger: logger,
	}
	return c.build()
}

func (m command) build() *cli.Command {
	args := args{}
	configCmd := cli.Command{
		Name:  "config",
		Usage: "A collection of config-related utilities for the Intel Gaudi Container Toolkit",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return m.run(&args)
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "dry-run",
				Usage:       "update the runtime configuration as required but don't write changes to disk",
				Destination: &args.dryRun,
			},
			&cli.StringFlag{
				Name:        "config",
				Usage:       "path to the config file for the target runtime",
				Destination: &args.configFilePath,
				Value:       config.DefaultConfigPath,
			},
			&cli.StringSliceFlag{
				Name: "set",
				Usage: "Set a config value using the pattern 'key[=value]'. " +
					"Specifying only 'key' is equivalent to 'key=true' for boolean settings. " +
					"This flag can be specified multiple times, but only the last value for a specific " +
					"config option is applied. " +
					"If the setting represents a list, the elements are comma-separated.",
				Destination: &args.sets,
			},
		},
	}

	return &configCmd
}

func (m command) run(args *args) error {
	cfg := config.DefaultConfig()
	cfgToml, err := utils.NewToml(
		utils.TomlFromStruct(cfg),
	)
	if err != nil {
		return fmt.Errorf("unable to create config: %v", err)
	}

	for _, set := range args.sets {
		key, value, err := parseSetOption(set)
		if err != nil {
			return fmt.Errorf("invalid --set option %v: %w", set, err)
		}
		if value == nil {
			err = cfgToml.DeletePath(key)
			if err != nil {
				return fmt.Errorf("unable to delete config path %v: %w", key, err)
			}
		} else {
			err = cfgToml.SetPath(key, value)
			if err != nil {
				return fmt.Errorf("unable to set config path %v to %v: %w", key, value, err)
			}
		}
	}

	if args.dryRun {
		m.logger.Info("Dry run mode enabled, not saving changes to disk")
		output, err := cfgToml.Serialize()
		if err != nil {
			return fmt.Errorf("unable to serialize configuration: %v", err)
		}
		m.logger.Infof("Dry run output:\n%s", string(output))
		return nil
	}

	// Save the updated configuration to the specified file path
	m.logger.Infof("Saving configuration to %s", args.configFilePath)
	err = cfgToml.Save(args.configFilePath)
	if err != nil {
		return fmt.Errorf("unable to flush config: %v", err)
	}

	return nil
}

func parseSetOption(set string) ([]string, interface{}, error) {
	var key, valStr string
	eqIdx := -1
	for i, c := range set {
		if c == '=' {
			eqIdx = i
			break
		}
	}
	if eqIdx == -1 {
		key = set
		valStr = ""
	} else {
		key = set[:eqIdx]
		valStr = set[eqIdx+1:]
	}
	if key == "" {
		return nil, nil, fmt.Errorf("empty key in set option")
	}
	keys := strings.Split(key, ".")

	// If no value is provided, treat as boolean true
	if eqIdx == -1 {
		return keys, true, nil
	}

	// Try to parse value as bool, int, float, or fallback to string
	if valStr == "true" {
		return keys, true, nil
	}
	if valStr == "false" {
		return keys, false, nil
	}
	// Try int
	var intVal int
	_, err := fmt.Sscanf(valStr, "%d", &intVal)
	if err == nil {
		return keys, intVal, nil
	}
	// Try float
	var floatVal float64
	_, err = fmt.Sscanf(valStr, "%f", &floatVal)
	if err == nil {
		return keys, floatVal, nil
	}
	// Try comma-separated list
	if len(valStr) > 0 && strings.Contains(valStr, ",") {
		parts := strings.Split(valStr, ",")
		return keys, parts, nil
	}
	return keys, valStr, nil
}
