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
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/HabanaAI/habana-container-runtime/internal/config"
	"github.com/HabanaAI/habana-container-runtime/internal/discover"
	"github.com/HabanaAI/habana-container-runtime/internal/netinfo"

	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	hookDefaultFilePath = "/usr/bin/habana-container-hook"
	defaultL3Config     = "/etc/habanalabs/gaudinet.json"
)

const (
	EnvHLVisibleDevices   = "HABANA_VISIBLE_DEVICES"
	EnvHLVisibleModules   = "HABANA_VISIBLE_MODULES"
	EnvHLVisiblePciBusIds = "HABANA_VISIBLE_PCI_BUS_IDS"
	EnvHLRuntimeError     = "HABANA_RUNTIME_ERROR"
)

var (
	execRunc       = execRuncFunc
	resolveDevices = discover.ResolveDevices
	execLookPath   = exec.LookPath
	osStat         = os.Stat
	osExecutable   = os.Executable
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		_, err := fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		if err != nil {
			log.Printf("failed to write error to stderr: %v", err)
		}
		os.Exit(1)
	}

	logFile, err := os.OpenFile(cfg.Runtime.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		_, err := fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		if err != nil {
			log.Printf("failed to write error to stderr: %v", err)
		}
		os.Exit(1)
	}
	defer logFile.Close()

	logger := slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: cfg.Runtime.LogLevel}))

	if err := run(logger, cfg, os.Args[1:]); err != nil {
		logger.Error(err.Error())
		_, err := fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		if err != nil {
			log.Printf("failed to write error to stderr: %v", err)
		}
		os.Exit(1)
	}
}

func run(logger *slog.Logger, cfg *config.Config, args []string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("runtime panic: %v", r)
			return
		}
	}()

	err = handleRequest(logger, cfg, args)
	if err != nil {
		return err
	}

	return execRunc(logger, args, cfg.Runtime.SystemdCgroup)
}

// handleRequest manages the flow of the incoming command. Based on the command type
// and container environment variable, we either skip everything altogether, or
// modify the container specs based on the provided environment variables.
func handleRequest(logger *slog.Logger, cfg *config.Config, args []string) error {
	// If has 'create' command', then need to modify runc
	// otherwide, no modification needed
	if !hasCreateCommand(args) {
		logger.Debug("Not a create command, skipping", "command", args)
		return nil
	}

	// In CDI mode, habana-container-runtime shouldn't be called. If it its,
	// we just continue without any modifications.
	if cfg.Runtime.Mode == config.ModeCDI {
		logger.Debug("Got called in CDI mode, nothing to do.", "mode", cfg.Runtime.Mode)
		return nil
	}

	bundleDir, err := parseBundle(args)
	if err != nil {
		return fmt.Errorf("parsing bundle: %w", err)
	}
	logger.Debug("Bundle directory", "path", bundleDir)

	// Load specs
	bundleConfigFile := bundleDir + "/config.json"
	logger.Info("Using bundle file", "path", bundleConfigFile)

	specConfig, err := loadSpecs(bundleConfigFile)
	if err != nil {
		return fmt.Errorf("loading specs: %w", err)
	}
	requestedDevices, err := resolveDevices(specConfig, cfg.AllowEnvDeviceRequests, cfg.Runtime.AlwaysMount)
	if err != nil {
		return fmt.Errorf("authorizing Habana devices: %w", err)
	}
	if len(requestedDevices) == 0 {
		return nil
	}
	if specConfig.Root == nil || specConfig.Root.Path == "" {
		return fmt.Errorf("OCI root filesystem is missing")
	}
	if specConfig.Linux == nil {
		specConfig.Linux = &specs.Linux{}
	}
	if specConfig.Linux.Resources == nil {
		specConfig.Linux.Resources = &specs.LinuxResources{}
	}

	// If legacy mode, add habana-hook as a prestart hook, and return to
	// execute runc. The hook and libhabana takes cares of the devices mounts.
	if cfg.Runtime.Mode == config.ModeLegacy {
		logger.Info("In legacy mode")
		err = addPrestartHook(logger, specConfig, cfg, requestedDevices)
		if err != nil {
			return fmt.Errorf("adding habana prestart hook: %s", err)
		}
		return saveSpecs(bundleConfigFile, specConfig)
	}

	logger.Debug("Requested devices", "devices", requestedDevices)

	if cfg.AllowEnvDeviceRequests && cfg.MountAccelerators {
		err = addAcceleratorDevices(logger, specConfig, requestedDevices)
		if err != nil {
			addErrorEnvVar(specConfig, err.Error())
			return fmt.Errorf("adding accelerator devices: %w", err)
		}
	}

	err = addAcceleratorModuleIDs(logger, specConfig, requestedDevices)
	if err != nil {
		addErrorEnvVar(specConfig, err.Error())
		return fmt.Errorf("adding modules_ids env var: %w", err)
	}

	if cfg.MountUverbs {
		err = addUverbsDevices(logger, specConfig, requestedDevices)
		if err != nil {
			addErrorEnvVar(specConfig, err.Error())
			return fmt.Errorf("adding uverb devices: %w", err)
		}
	}
	err = addCreateRuntimeHook(logger, specConfig, cfg, requestedDevices)
	if err != nil {
		return fmt.Errorf("adding createRuntime hook: %w", err)
	}

	// Docker saves the abolute path while containerd mentions the folder name
	// relative to the bundle dir.
	containerRootFS := path.Join(bundleDir, specConfig.Root.Path)
	if path.IsAbs(specConfig.Root.Path) {
		containerRootFS = specConfig.Root.Path
	}

	err = netinfo.Generate(requestedDevices, containerRootFS)
	if err != nil {
		addErrorEnvVar(specConfig, err.Error())
		logger.Error(fmt.Sprintf("generating macAddrInfo failed: %v", err))
	}

	err = netinfo.GaudinetFile(logger, containerRootFS, cfg.NetworkL3Config.Path)
	if err != nil {
		addErrorEnvVar(specConfig, err.Error())
		logger.Error(fmt.Sprintf("generating gaudinet file failed: %v", err))
	}

	return saveSpecs(bundleConfigFile, specConfig)
}

func execRuncFunc(logger *slog.Logger, args []string, systemdCgroup bool) error {
	logger.Debug("Looking for 'docker-runc' in PATH")
	runcPath, err := exec.LookPath("docker-runc")
	if err != nil {
		runcPath, err = exec.LookPath("runc")
		if err != nil {
			return err
		}
	}
	logger.Debug("runc path", "path", runcPath)
	cmdArgs := []string{runcPath}
	if systemdCgroup {
		cmdArgs = append(cmdArgs, "--systemd-cgroup")
	}
	cmdArgs = append(cmdArgs, args...)
	logger.Debug("Executing runc command", "cmd", cmdArgs)

	return syscall.Exec(runcPath, cmdArgs, os.Environ())
}

func hasCreateCommand(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if arg == "create" {
			return true
		}
	}
	return false
}

func IsHabanaContainer(spec *specs.Spec) bool {
	if spec.Process == nil {
		return false
	}
	for _, ev := range spec.Process.Env {
		key, _, found := strings.Cut(ev, "=")
		if found && (key == EnvHLVisibleDevices || key == EnvHLVisibleModules || key == EnvHLVisiblePciBusIds) {
			return true
		}
	}
	return false
}

func addErrorEnvVar(spec *specs.Spec, msg string) {
	for _, env := range spec.Process.Env {
		if strings.HasPrefix(env, EnvHLRuntimeError) {
			return
		}
	}
	addEnvVar(spec, EnvHLRuntimeError, strconv.Quote(msg))
}
