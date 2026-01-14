package docker

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/HabanaAI/habana-container-runtime/internal/utils"
	"github.com/sirupsen/logrus"
)

type DockerConfig struct {
	logger   *logrus.Logger
	config   *map[string]interface{}
	checksum string
}

type Option func(*DockerConfig) error

func New(opts ...Option) (*DockerConfig, error) {
	cfg := &DockerConfig{
		logger: logrus.New(),
		config: &map[string]interface{}{},
	}

	for _, opt := range opts {
		err := opt(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to apply option: %v", err)
		}
	}

	serializedData, err := cfg.Serialize()
	if err != nil {
		return nil, fmt.Errorf("unable to serialize configuration: %v", err)
	}
	cfg.checksum = utils.ComputeChecksum(serializedData)

	return cfg, nil
}

func WithLogger(logger *logrus.Logger) Option {
	return func(b *DockerConfig) error {
		b.logger = logger
		return nil
	}
}

func FromConfigPath(path string) Option {
	return func(c *DockerConfig) error {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			c.config = &map[string]interface{}{}
			c.logger.Warnf("configuration file does not exist: %s, using empty config", path)
			return nil
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read configuration file: %v", err)
		}

		err = FromBytes(raw)(c)
		if err != nil {
			return fmt.Errorf("failed to load configuration from path: %v", err)
		}

		c.logger.Infof("Loaded configuration from %s", path)
		return nil
	}
}

func FromBytes(data []byte) Option {
	return func(c *DockerConfig) error {
		var config map[string]interface{}
		err := json.Unmarshal(data, &config)
		if err != nil {
			return fmt.Errorf("failed to load configuration from string: %v", err)
		}
		c.config = &config

		c.logger.Infof("Loaded configuration from string")
		return nil
	}
}

func (c *DockerConfig) AddRuntime(binariesDir string) error {
	if c.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	config := *c.config

	runtimes := make(map[string]interface{})
	if val, exists := config["runtimes"]; exists {
		if rt, ok := val.(map[string]interface{}); ok {
			runtimes = rt
		} else {
			c.logger.Warnf("config['runtimes'] is not a map[string]interface{}, overwriting with new map")
		}
	}

	runtimes["habana"] = map[string]interface{}{
		"path": fmt.Sprintf("%s/habana-container-runtime", binariesDir),
		"args": []string{},
	}

	config["runtimes"] = runtimes

	*c.config = config
	c.logger.Infof("Runtime added successfully")
	return nil
}

func (c *DockerConfig) EnableCDI() error {
	if c.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	config := *c.config

	features, ok := config["features"].(map[string]bool)
	if !ok {
		features = make(map[string]bool)
	}
	features["cdi"] = true

	config["features"] = features

	*c.config = config
	c.logger.Infof("CDI enabled successfully")
	return nil
}

func (c *DockerConfig) RemoveRuntime() error {
	if c.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	config := *c.config

	if val, exists := config["runtimes"]; exists {
		if runtimes, ok := val.(map[string]interface{}); ok {

			delete(runtimes, "habana")

			if len(runtimes) == 0 {
				delete(config, "runtimes")
			}
		} else {
			c.logger.Warnf("Expected 'runtimes' to be map[string]interface{}, but got %T", val)
		}
	}

	*c.config = config

	c.logger.Infof("Runtime removed successfully")
	return nil
}

func (c *DockerConfig) Serialize() ([]byte, error) {
	if c.config == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	output, err := json.MarshalIndent(c.config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("unable to convert to JSON: %v", err)
	}

	c.logger.Infof("Configuration serialized successfully")
	return output, nil
}

func (c *DockerConfig) Save(path string) error {
	output, err := c.Serialize()
	if err != nil {
		return fmt.Errorf("unable to serialize configuration: %v", err)
	}

	// Save to file
	err = os.WriteFile(path, output, 0640)
	if err != nil {
		return fmt.Errorf("unable to write configuration file: %v", err)
	}
	c.logger.Infof("Configuration saved successfully")
	return nil
}

func (c *DockerConfig) IsModified() (bool, error) {
	if c.config == nil {
		return false, fmt.Errorf("no configuration loaded")
	}

	serializedData, err := c.Serialize()
	if err != nil {
		return false, fmt.Errorf("unable to serialize configuration: %v", err)
	}
	currentChecksum := utils.ComputeChecksum(serializedData)
	return currentChecksum != c.checksum, nil
}

func (c *DockerConfig) RestartRuntime() error {
	pid, err := utils.GetProcessPid("dockerd")
	if err != nil {
		return fmt.Errorf("unable to get crio pid: %v", err)
	}

	_, err = utils.RunCommandInHostNamespace([]string{"kill", "-1", pid})
	if err != nil {
		return fmt.Errorf("unable to restart docker: %v", err)
	}
	c.logger.Infof("Docker restarted successfully with pid %s", pid)

	return nil
}
