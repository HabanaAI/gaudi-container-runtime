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
	"reflect"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestEnvironmentDeviceRequestsRequireOptIn(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.AllowEnvDeviceRequests {
		t.Fatal("environment-only device requests must be disabled by default")
	}
	if err := toml.Unmarshal([]byte("accept-habana-visible-devices-envvar-when-unprivileged = true\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.AllowEnvDeviceRequests {
		t.Fatal("legacy unprivileged setting enabled environment-only allocation")
	}
	if err := toml.Unmarshal([]byte("allow-env-device-requests = true\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.AllowEnvDeviceRequests {
		t.Fatal("explicit standalone compatibility setting was ignored")
	}
}

func TestGetConfig(t *testing.T) {
	want := &Config{
		AcceptEnvvarUnprivileged: true,
		MountAccelerators:        true,
		MountUverbs:              false,
		BinariesDir:              "/usr/local/bin",
		Runtime: RuntimeConfig{
			AlwaysMount:   false,
			LogFile:       "/tmp/runtime-test",
			LogLevel:      slog.LevelDebug,
			SystemdCgroup: true,
			Mode:          ModeLegacy,
		},
		CLI: CLIConfig{
			LogFile:     "/var/log/habana-container-hook.log",
			Root:        nil,
			Path:        nil,
			Environment: []string{},
		},
		NetworkL3Config: NetworkConfig{
			"/tmp/testdata.json",
		},
	}
	t.Setenv("XDG_CONFIG_HOME", "testdata/input")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() err=%q, want nil", err)
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("got %+v\nwant %+v", cfg, want)
	}
}
