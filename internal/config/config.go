/*
 * Copyright (c) 2022, HabanaLabs Ltd.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package config

import (
	"log/slog"
	"os"
	"path"

	"github.com/pelletier/go-toml/v2"
)

const (
	DefaultConfigPath = "/etc/habana-container-runtime/config.toml"
	driverPath        = "/run/habana/driver"
	configOverride    = "XDG_CONFIG_HOME"
	configFilePath    = "habana-container-runtime/config.toml"

	hookDefaultFilePath = "/usr/bin/habana-container-hook"
	defaultL3Config     = "/etc/habanalabs/gaudinet.json"
)

const (
	ModeOCI    string = "oci"
	ModeLegacy string = "legacy"
	ModeCDI    string = "cdi"
)

var configDir = "/etc/"

type Config struct {
	AcceptEnvvarUnprivileged bool          `toml:"accept-habana-visible-devices-envvar-when-unprivileged"`
	BinariesDir              string        `toml:"binaries-dir"`
	CLI                      CLIConfig     `toml:"habana-container-cli"`
	MountAccelerators        bool          `toml:"mount_accelerators"`
	MountUverbs              bool          `toml:"mount_uverbs"`
	NetworkL3Config          NetworkConfig `toml:"network-layer-routes"`
	Runtime                  RuntimeConfig `toml:"habana-container-runtime"`
}

type NetworkConfig struct {
	Path string `toml:"path"`
}

type RuntimeConfig struct {
	AlwaysMount   bool       `toml:"visible_devices_all_as_default"`
	LogFile       string     `toml:"log_file"`
	LogLevel      slog.Level `toml:"log_level"`
	Mode          string     `toml:"mode"`
	SystemdCgroup bool       `toml:"systemd_cgroup"`
}

type CLIConfig struct {
	Environment []string   `toml:"environment"`
	LogFile     string     `toml:"log_file"`
	LogLevel    slog.Level `toml:"log_level"`
	Path        *string    `toml:"path"`
	Root        *string    `toml:"root"`
}

func Load() (*Config, error) {
	cfg := DefaultConfig()

	if XDGConfigDir := os.Getenv(configOverride); len(XDGConfigDir) != 0 {
		configDir = XDGConfigDir
	}
	configFilePath := path.Join(configDir, configFilePath)

	f, err := os.Open(configFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	err = toml.NewDecoder(f).Decode(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func DefaultConfig() Config {
	return Config{
		AcceptEnvvarUnprivileged: true,
		MountAccelerators:        true,
		MountUverbs:              true,
		BinariesDir:              "/usr/local/bin",
		NetworkL3Config: NetworkConfig{
			Path: defaultL3Config,
		},
		Runtime: RuntimeConfig{
			AlwaysMount:   false,
			LogFile:       "/var/log/habana-container-runtime.log",
			LogLevel:      slog.LevelInfo,
			SystemdCgroup: false,
			Mode:          ModeOCI,
		},
		CLI: CLIConfig{
			Root:        nil,
			Path:        nil,
			Environment: []string{},
			LogFile:     "/var/log/habana-container-hook.log",
		},
	}
}
