package crio

import (
	"fmt"

	"github.com/HabanaAI/habana-container-runtime/internal/utils"
	"github.com/sirupsen/logrus"
)

type CRIOConfig struct {
	logger   *logrus.Logger
	config   *utils.TomlConfig[map[string]interface{}]
	checksum string
}

type Option func(*CRIOConfig) error

func New(opts ...Option) (*CRIOConfig, error) {
	cfg := &CRIOConfig{
		logger: logrus.New(),
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
	return func(b *CRIOConfig) error {
		b.logger = logger
		return nil
	}
}

func FromConfigPath(path string) Option {
	return func(c *CRIOConfig) error {
		tomlConfig, err := utils.NewToml(utils.TomlFromConfigPath[map[string]interface{}](path))
		if err != nil {
			return fmt.Errorf("failed to load configuration from path: %v", err)
		}
		c.config = tomlConfig

		c.logger.Infof("Loaded configuration from path: %s", path)
		return nil
	}
}

func FromString(data string) Option {
	return func(c *CRIOConfig) error {
		tomlConfig, err := utils.NewToml(utils.TomlFromByte[map[string]interface{}]([]byte(data)))
		if err != nil {
			return fmt.Errorf("failed to load configuration from string: %v", err)
		}
		c.config = tomlConfig

		c.logger.Infof("Loaded configuration from string")
		return nil
	}
}

func (c *CRIOConfig) AddRuntime(binariesDir string) error {
	if c.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	pathPrefix := []string{"crio", "runtime", "runtimes", "habana"}
	err := c.config.SetPath(append(pathPrefix, "runtime_path"), fmt.Sprintf("%s/habana-container-runtime", binariesDir))
	if err != nil {
		return fmt.Errorf("unable to set runtime_path: %v", err)
	}
	err = c.config.SetPath(append(pathPrefix, "monitor_path"), "/usr/libexec/crio/conmon")
	if err != nil {
		return fmt.Errorf("unable to set monitor_path: %v", err)
	}
	err = c.config.SetPath(append(pathPrefix, "monitor_env"), []string{
		fmt.Sprintf("PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:%s", binariesDir),
	})
	if err != nil {
		return fmt.Errorf("unable to set monitor_env: %v", err)
	}

	c.logger.Infof("Runtime added successfully")
	return nil
}

func (c *CRIOConfig) EnableCDI() error {
	if c.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	err := c.config.SetPath([]string{"crio", "runtime", "cdi_spec_dirs"}, []string{"/etc/cdi", "/var/run/cdi"})
	if err != nil {
		return fmt.Errorf("unable to set CDI spec dirs: %v", err)
	}

	c.logger.Infof("CDI enabled successfully")
	return nil
}

func (c *CRIOConfig) RemoveRuntime() error {
	if c.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	path := []string{"crio", "runtime", "runtimes", "habana"}
	err := c.config.DeletePath(path)
	if err != nil {
		return fmt.Errorf("unable to remove runtime: %v", err)
	}

	c.logger.Infof("Runtime removed successfully")
	return nil
}

func (c *CRIOConfig) Serialize() ([]byte, error) {
	if c.config == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	output, err := c.config.Serialize()
	if err != nil {
		return nil, fmt.Errorf("unable to convert to TOML: %v", err)
	}

	c.logger.Infof("Configuration serialized successfully")
	return output, nil
}

func (c *CRIOConfig) Save(path string) error {
	if err := c.config.Save(path); err != nil {
		return fmt.Errorf("failed to save configuration: %v", err)
	}
	c.logger.Infof("Configuration saved successfully")
	return nil
}

func (c *CRIOConfig) IsModified() (bool, error) {
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

func (c *CRIOConfig) RestartRuntime() error {
	pid, err := utils.GetProcessPid("crio")
	if err != nil {
		return fmt.Errorf("unable to get crio pid: %v", err)
	}

	_, err = utils.RunCommandInHostNamespace([]string{"kill", "-1", pid})
	if err != nil {
		return fmt.Errorf("unable to restart crio: %v", err)
	}
	c.logger.Infof("CRI-O restarted successfully with pid %s", pid)
	return nil
}
