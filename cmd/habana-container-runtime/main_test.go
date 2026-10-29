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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/HabanaAI/habana-container-runtime/internal/config"
	"github.com/HabanaAI/habana-container-runtime/internal/discover"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

func TestAllocatedContainerPreservesAcceleratorGrants(t *testing.T) {
	originalResolve, originalExec, originalLookPath := resolveDevices, execRunc, execLookPath
	originalSysClass := discover.SysClassAccel
	t.Cleanup(func() {
		resolveDevices, execRunc, execLookPath = originalResolve, originalExec, originalLookPath
		discover.SysClassAccel = originalSysClass
	})
	inventory := []*discover.DevInfo{
		{Path: "/dev/accel/accel10", Major: 261, Minor: 10, Mode: unix.S_IFCHR},
		{Path: "/dev/accel/accel0", Major: 261, Minor: 0, Mode: unix.S_IFCHR},
	}
	resolveDevices = func(spec *specs.Spec, allowEnvironment, alwaysMount bool) ([]string, error) {
		if allowEnvironment {
			t.Fatal("default runtime enabled environment-only allocation")
		}
		return discover.SelectDevices(spec.Process.Env, discover.AllocatedDevices(spec, inventory), alwaysMount)
	}
	execLookPath = func(string) (string, error) { return "/trusted/habana-container-hook", nil }
	discover.SysClassAccel = filepath.Join(t.TempDir(), "accel")
	if err := os.MkdirAll(discover.SysClassAccel+"10/device", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(discover.SysClassAccel+"10/device/module_id", []byte("3"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{config.ModeOCI, config.ModeLegacy} {
		for _, selector := range []string{"all", "10", "0"} {
			t.Run(mode+"/"+selector, func(t *testing.T) {
				bundle := t.TempDir()
				major, minor := int64(261), int64(10)
				spec := &specs.Spec{
					Version: "1.0.2",
					Process: &specs.Process{Env: []string{"HABANA_VISIBLE_DEVICES=" + selector}},
					Root:    &specs.Root{Path: "rootfs"},
					Linux: &specs.Linux{
						Devices:   []specs.LinuxDevice{{Path: "/dev/accel/accel10", Type: "c", Major: major, Minor: minor}},
						Resources: &specs.LinuxResources{Devices: []specs.LinuxDeviceCgroup{{Allow: true, Type: "c", Major: &major, Minor: &minor, Access: "rw"}}},
					},
				}
				before, err := json.Marshal(spec)
				if err != nil {
					t.Fatal(err)
				}
				bundleFile := filepath.Join(bundle, "config.json")
				if err := os.WriteFile(bundleFile, before, 0600); err != nil {
					t.Fatal(err)
				}
				called := false
				execRunc = func(*slog.Logger, []string, bool) error { called = true; return nil }
				cfg := config.DefaultConfig()
				cfg.Runtime.Mode, cfg.MountUverbs = mode, false
				err = run(slog.New(slog.NewTextHandler(io.Discard, nil)), &cfg, []string{"create", "--bundle", bundle, "test"})
				if selector == "0" {
					after, readErr := os.ReadFile(bundleFile)
					if err == nil || called || readErr != nil || !bytes.Equal(before, after) {
						t.Fatalf("unauthorized device did not fail closed: %v", err)
					}
					return
				}
				if err != nil || !called {
					t.Fatalf("allocated container failed: %v", err)
				}
				got, err := loadSpecs(bundleFile)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got.Linux, spec.Linux) {
					t.Fatal("runtime expanded the accelerator device grants")
				}
				hooks := got.Hooks.CreateRuntime
				if mode == config.ModeLegacy {
					hooks = got.Hooks.Prestart
				}
				if len(hooks) != 1 || hooks[0].Args[1] != "--devices=10" {
					t.Fatalf("allocated hook selection = %v", hooks)
				}
			})
		}
	}
}

func TestUnallocatedContainerIsUnchanged(t *testing.T) {
	original := execRunc
	t.Cleanup(func() { execRunc = original })
	for _, mode := range []string{config.ModeOCI, config.ModeLegacy, config.ModeCDI} {
		for _, mountAccelerators := range []bool{false, true} {
			for _, selector := range []string{"", "HABANA_VISIBLE_DEVICES=all", "HABANA_VISIBLE_DEVICES=0", "HABANA_VISIBLE_MODULES=all", "HABANA_VISIBLE_PCI_BUS_IDS=all", "HABANA_VISIBLE_DEVICES_OTHER=all"} {
				t.Run(mode+"/"+selector+"/mount="+fmt.Sprint(mountAccelerators), func(t *testing.T) {
					cfg := config.DefaultConfig()
					cfg.Runtime.Mode = mode
					cfg.Runtime.AlwaysMount = true
					cfg.MountAccelerators = mountAccelerators
					spec := &specs.Spec{Process: &specs.Process{Env: []string{selector}}, Root: &specs.Root{Path: "rootfs"}}
					before, err := json.MarshalIndent(spec, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					bundle := t.TempDir()
					bundleFile := filepath.Join(bundle, "config.json")
					if err := os.WriteFile(bundleFile, before, 0600); err != nil {
						t.Fatal(err)
					}
					called := false
					execRunc = func(*slog.Logger, []string, bool) error { called = true; return nil }
					if err := run(slog.New(slog.NewTextHandler(io.Discard, nil)), &cfg, []string{"create", "--bundle", bundle, "test"}); err != nil {
						t.Fatal(err)
					}
					after, err := os.ReadFile(bundleFile)
					if err != nil || !bytes.Equal(before, after) || !called {
						t.Fatalf("unallocated container changed or runc skipped: called=%v, err=%v", called, err)
					}
				})
			}
		}
	}
}

func TestAuthorizationFailureDoesNotExecuteRunc(t *testing.T) {
	original := execRunc
	t.Cleanup(func() { execRunc = original })
	execRunc = func(*slog.Logger, []string, bool) error {
		t.Fatal("runc executed after authorization failure")
		return nil
	}
	bundle := t.TempDir()
	before := []byte(`{"ociVersion":"1.0.2","root":{"path":"rootfs"}}`)
	bundleFile := filepath.Join(bundle, "config.json")
	if err := os.WriteFile(bundleFile, before, 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	err := run(slog.New(slog.NewTextHandler(io.Discard, nil)), &cfg, []string{"create", "--bundle", bundle, "test"})
	if err == nil {
		t.Fatal("missing process was accepted")
	}
	after, err := os.ReadFile(bundleFile)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("authorization failure modified the bundle")
	}
}

func TestHasCreateCommand(t *testing.T) {
	t.Parallel()

	createArgs := `--root /run/containerd/runc/k8s.io --log /run/containerd/io.containerd.runtime.v2.task/k8s.io/258cfa8cbc7edee9a846fe691971aa7c35976ea29bc668e7da2692e940032a88/log.json --log-format json create --bundle ./testdata/input --pid-file /run/containerd/io.containerd.runtime.v2.task/k8s.io/258cfa8cbc7edee9a846fe691971aa7c35976ea29bc668e7da2692e940032a88/init.pid 258cfa8cbc7edee9a846fe691971aa7c35976ea29bc668e7da2692e940032a88`
	deleteArgs := `--root /run/containerd/runc/k8s.io --log /run/containerd/io.containerd.runtime.v2.task/k8s.io/258cfa8cbc7edee9a846fe691971aa7c35976ea29bc668e7da2692e940032a88/log.json --log-format json delete --force 258cfa8cbc7edee9a846fe691971aa7c35976ea29bc668e7da2692e940032a88`
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{
			name: "with create command",
			in:   createArgs,
			want: true,
		},
		{
			name: "with delete command",
			in:   deleteArgs,
			want: false,
		},
		{
			name: "with create flags but not command",
			in:   "--bla --create -create test foo bar",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasCreateCommand(strings.Fields(tt.in)); got != tt.want {
				t.Errorf("want %t, got %t", got, tt.want)
			}
		})
	}
}
