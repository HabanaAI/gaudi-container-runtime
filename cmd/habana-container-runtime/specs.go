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
package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/HabanaAI/habana-container-runtime/internal/config"
	"github.com/HabanaAI/habana-container-runtime/internal/discover"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func loadSpecs(bundleConfigFile string) (*specs.Spec, error) {
	jsonFile, err := os.Open(filepath.Clean(bundleConfigFile))
	if err != nil {
		return nil, fmt.Errorf("opening OCI spec file: %w", err)
	}
	defer jsonFile.Close()

	var spec specs.Spec
	err = json.NewDecoder(jsonFile).Decode(&spec)
	if err != nil {
		return nil, fmt.Errorf("reading OCI spec file: %w", err)
	}

	return &spec, nil
}

func saveSpecs(bundleConfigFile string, spec *specs.Spec) error {
	jsonOutput, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshaling OCI spec: %w", err)
	}
	info, err := os.Stat(bundleConfigFile)
	if err != nil {
		return fmt.Errorf("stat OCI spec file: %w", err)
	}
	jsonFile, err := os.CreateTemp(filepath.Dir(bundleConfigFile), ".habana-config-*")
	if err != nil {
		return fmt.Errorf("creating OCI spec file: %w", err)
	}
	defer os.Remove(jsonFile.Name())
	defer jsonFile.Close()
	if err := jsonFile.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if _, err := jsonFile.Write(jsonOutput); err != nil {
		return fmt.Errorf("writing OCI spec file: %w", err)
	}
	if err := jsonFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(jsonFile.Name(), bundleConfigFile); err != nil {
		return fmt.Errorf("replacing OCI spec file: %w", err)
	}
	return nil
}

func addPrestartHook(logger *slog.Logger, spec *specs.Spec, cfg *config.Config, requestedDevs []string) error {
	// path, err := execLookPath("habana-container-runtime-hook")
	path, err := hookBinaryPath(cfg)
	if err != nil {
		path = hookDefaultFilePath
		_, err = os.Stat(path)
		if err != nil {
			return err
		}
	}
	logger.Info("Prestart hook path", "path", path)

	args := []string{path}
	if spec.Hooks == nil {
		spec.Hooks = &specs.Hooks{}
	} else if len(spec.Hooks.Prestart) != 0 {
		found := false
		for index, hook := range spec.Hooks.Prestart {
			if !strings.Contains(hook.Path, "habana-container-hook") {
				continue
			}
			spec.Hooks.Prestart[index].Path = path
			spec.Hooks.Prestart[index].Args = append(args, "--devices="+strings.Join(requestedDevs, ","), "prestart")
			found = true
		}
		if found {
			return nil
		}
	}

	spec.Hooks.Prestart = append(spec.Hooks.Prestart, specs.Hook{
		Path: path,
		Args: append(args, "--devices="+strings.Join(requestedDevs, ","), "prestart"),
	})

	logger.Info("Prestart hook added, executing runc")
	return nil
}

func addCreateRuntimeHook(logger *slog.Logger, spec *specs.Spec, cfg *config.Config, requestedDevs []string) error {
	path, err := hookBinaryPath(cfg)
	if err != nil {
		path = hookDefaultFilePath
		_, err = os.Stat(path)
		if err != nil {
			return err
		}
	}
	logger.Info("hook binary path", "path", path)

	args := []string{path}
	if spec.Hooks == nil {
		spec.Hooks = &specs.Hooks{}
	} else if len(spec.Hooks.CreateRuntime) != 0 {
		found := false
		for index, hook := range spec.Hooks.CreateRuntime {
			if !strings.Contains(hook.Path, "habana-container-hook") {
				continue
			}
			spec.Hooks.CreateRuntime[index].Path = path
			spec.Hooks.CreateRuntime[index].Args = append(args, "--devices="+strings.Join(requestedDevs, ","), "createRuntime")
			found = true
		}
		if found {
			return nil
		}
	}

	spec.Hooks.CreateRuntime = append(spec.Hooks.CreateRuntime, specs.Hook{
		Path: path,
		Args: append(args, "--devices="+strings.Join(requestedDevs, ","), "createRuntime"),
	})

	logger.Info("createRuntime hook added, executing runc")
	return nil
}

func addAcceleratorDevices(logger *slog.Logger, spec *specs.Spec, requestedDevs []string) error {
	logger.Debug("Discovering accelerators")

	// Prepare devices in OCI format
	var devs []*discover.DevInfo
	for _, reqDev := range requestedDevs {
		for _, d := range discover.AcceleratorDevices() {
			id, valid := discover.AcceleratorID(d)
			if !valid || id != reqDev {
				continue
			}
			logger.Info("Adding accelerator device", "path", d)
			i, err := discover.DeviceInfo(d)
			if err != nil {
				return err
			}
			devs = append(devs, i)
		}
	}
	addDevicesToSpec(logger, spec, devs)
	addAllowList(logger, spec, devs)

	return nil
}

func addAcceleratorModuleIDs(logger *slog.Logger, spec *specs.Spec, requestedDevs []string) error {
	// Extract module id for HABANA_VISIBLE_MODULES environment variables
	modulesIDs := make([]string, 0, len(requestedDevs))
	for _, acc := range requestedDevs {
		id, err := discover.AcceleratorModuleID(acc)
		if err != nil {
			logger.Debug("discoring modules", "error", err)
			return err
		}
		modulesIDs = append(modulesIDs, id)
	}
	sort.Strings(modulesIDs)

	addEnvVar(spec, EnvHLVisibleModules, strings.Join(modulesIDs, ","))

	logger.Debug("Exposed module ids", "module_ids", modulesIDs)

	return nil
}

func addUverbsDevices(logger *slog.Logger, spec *specs.Spec, requestedDevs []string) error {
	logger.Debug("Discovering uverbs")

	var devs []*discover.DevInfo
	uverbs, err := discover.UverbsForAccelerators(requestedDevs)
	if err != nil {
		return err
	}
	for _, uverbDev := range uverbs {
		logger.Info("Adding uverb device", "path", uverbDev)
		i, err := discover.DeviceInfo(uverbDev)
		if err != nil {
			return err
		}
		logger.Info("Adding uverb device", "path", uverbDev)
		devs = append(devs, i)
	}

	addDevicesToSpec(logger, spec, devs)
	addAllowList(logger, spec, devs)

	return nil
}

// addDevicesToSpec adds list of devices nodes to be created for container.
func addDevicesToSpec(logger *slog.Logger, spec *specs.Spec, devices []*discover.DevInfo) {
	logger.Debug("Mounting devices in spec")
	current := make(map[string]struct{})

	for _, dev := range spec.Linux.Devices {
		current[dev.Path] = struct{}{}
	}

	var devicesToAdd []specs.LinuxDevice
	for _, hlDevice := range devices {
		if _, ok := current[hlDevice.Path]; ok {
			continue
		}

		zeroID := uint32(0)
		devicesToAdd = append(devicesToAdd, specs.LinuxDevice{
			Type:     "c",
			Major:    int64(hlDevice.Major),
			Minor:    int64(hlDevice.Minor),
			FileMode: &hlDevice.FileMode,
			Path:     hlDevice.Path,
			GID:      &zeroID,
			UID:      &zeroID,
		})
		logger.Debug("Added device to spec", "path", hlDevice.Path)
	}

	spec.Linux.Devices = append(spec.Linux.Devices, devicesToAdd...)
}

// addAllowList modifies the Linux devices allow list to cgroup rules.
func addAllowList(logger *slog.Logger, spec *specs.Spec, devices []*discover.DevInfo) {
	logger.Debug("Adding devices to allow list")

	current := make(map[string]bool)
	for _, dev := range spec.Linux.Resources.Devices {
		if dev.Major != nil && dev.Minor != nil {
			current[fmt.Sprintf("%d-%d", *dev.Major, *dev.Minor)] = true
		}
	}

	var devsToAdd []specs.LinuxDeviceCgroup
	for _, hldev := range devices {
		k := fmt.Sprintf("%d-%d", hldev.Major, hldev.Minor)
		if _, ok := current[k]; !ok {
			major := int64(hldev.Major)
			minor := int64(hldev.Minor)
			devsToAdd = append(devsToAdd, specs.LinuxDeviceCgroup{
				Allow:  true,
				Type:   "c",
				Major:  &major,
				Minor:  &minor,
				Access: "rwm",
			})
			logger.Debug("Added device to allow list", "major", hldev.Major, "minor", hldev.Minor)
		}
	}

	// modify spec
	spec.Linux.Resources.Devices = append(spec.Linux.Resources.Devices, devsToAdd...)
}

func addEnvVar(spec *specs.Spec, key string, value string) {
	env := spec.Process.Env[:0]
	for _, entry := range spec.Process.Env {
		name, _, _ := strings.Cut(entry, "=")
		if name != key {
			env = append(env, entry)
		}
	}
	spec.Process.Env = append(env, fmt.Sprintf("%s=%v", key, value))
}

// hookBinaryPath looks for the binary in the following locations by order:
//
// 1. $PATH environment variable
//
// 2. Same directory of the runtime
//
// 3. binaries-dir value from config file
//
// 4. Default location
func hookBinaryPath(cfg *config.Config) (string, error) {
	// Search in PATH
	binPath, err := execLookPath("habana-container-hook")
	if err == nil { // IF NO ERROR
		return binPath, nil
	}

	// Search in the binary habana-container-runtime's dir
	currentExec, err := osExecutable()
	if err == nil { // IF NO ERROR
		currentDir := filepath.Dir(currentExec)
		binPath = path.Join(currentDir, "habana-container-hook")
		if _, err := osStat(binPath); err == nil { // IF NO ERROR
			return binPath, nil
		}
	}

	// Search in the dir provided by binaries-dir
	binPath = path.Join(cfg.BinariesDir, "habana-container-hook")
	if _, err := osStat(binPath); err == nil { // IF NO ERROR
		return binPath, nil
	}

	binPath = hookDefaultFilePath
	_, err = osStat(binPath)
	if err == nil { // IF NO ERROR
		return binPath, nil
	}
	return "", fmt.Errorf("habana-container-hook was not found on the system")
}
