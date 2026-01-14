package containerd

import (
	"fmt"

	"github.com/HabanaAI/habana-container-runtime/internal/utils"
	"github.com/sirupsen/logrus"
)

const (
	criRuntimePluginName = "io.containerd.grpc.v1.cri"
)

type ContainerdConfig struct {
	logger   *logrus.Logger
	config   *utils.TomlConfig[map[string]interface{}]
	checksum string
}

type Option func(*ContainerdConfig) error

func New(opts ...Option) (*ContainerdConfig, error) {
	cfg := &ContainerdConfig{
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
	return func(b *ContainerdConfig) error {
		b.logger = logger
		return nil
	}
}

func FromConfigPath(path string) Option {
	return func(c *ContainerdConfig) error {
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
	return func(c *ContainerdConfig) error {
		tomlConfig, err := utils.NewToml(utils.TomlFromByte[map[string]interface{}]([]byte(data)))
		if err != nil {
			return fmt.Errorf("failed to load configuration from string: %v", err)
		}
		c.config = tomlConfig

		c.logger.Infof("Loaded configuration from string")
		return nil
	}
}

func (c *ContainerdConfig) AddRuntime(binariesDir string) error {
	if c.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	pathPrefix := c.getPluginPath(criRuntimePluginName, []string{"runtimes", "habana"})
	err := c.config.SetPath(append(pathPrefix, "runtime_type"), "io.containerd.runc.v2")
	if err != nil {
		return fmt.Errorf("unable to set runtime_type: %v", err)
	}
	err = c.config.SetPath(append(pathPrefix, "options", "BinaryName"), fmt.Sprintf("%s/habana-container-runtime", binariesDir))
	if err != nil {
		return fmt.Errorf("unable to set BinaryName: %v", err)
	}
	err = c.config.SetPath(append(pathPrefix, "options", "SystemdCgroup"), true)
	if err != nil {
		return fmt.Errorf("unable to set SystemdCgroup: %v", err)
	}

	c.logger.Infof("Runtime added successfully")
	return nil
}

func (c *ContainerdConfig) EnableCDI() error {
	if c.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	err := c.config.SetPath(c.getPluginPath(criRuntimePluginName, []string{"enable_cdi"}), true)
	if err != nil {
		return fmt.Errorf("unable to enable CDI: %v", err)
	}
	err = c.config.SetPath(c.getPluginPath(criRuntimePluginName, []string{"cdi_spec_dirs"}), []string{"/etc/cdi", "/var/run/cdi"})
	if err != nil {
		return fmt.Errorf("unable to set CDI spec dirs: %v", err)
	}

	c.logger.Infof("CDI enabled successfully")
	return nil
}

func (c *ContainerdConfig) RemoveRuntime() error {
	if c.config == nil {
		return fmt.Errorf("no configuration loaded")
	}
	path := c.getPluginPath(criRuntimePluginName, []string{"runtimes", "habana"})
	err := c.config.DeletePath(path)
	if err != nil {
		return fmt.Errorf("unable to remove runtime: %v", err)
	}

	c.logger.Infof("Runtime removed successfully")
	return nil
}

func (c *ContainerdConfig) Serialize() ([]byte, error) {
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

func (c *ContainerdConfig) Save(path string) error {
	if err := c.config.Save(path); err != nil {
		return fmt.Errorf("failed to save configuration: %v", err)
	}
	c.logger.Infof("Configuration saved successfully")
	return nil
}

func (c *ContainerdConfig) IsModified() (bool, error) {
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

func (c *ContainerdConfig) RestartRuntime() error {
	_, err := utils.RunCommandInHostNamespace([]string{"systemctl", "restart", "containerd"})
	if err != nil {
		return fmt.Errorf("unable to restart containerd: %v", err)
	}
	c.logger.Infof("Containerd restarted successfully using systemctl")

	return nil
}

func (c *ContainerdConfig) getPluginPath(pluginName string, subpath []string) []string {
	path := []string{"plugins", pluginName}
	if len(subpath) > 0 {
		path = append(path, subpath...)
	}
	return path
}
