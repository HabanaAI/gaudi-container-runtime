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
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/HabanaAI/habana-container-runtime/internal/config"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestHookArguments(t *testing.T) {
	original := execLookPath
	t.Cleanup(func() { execLookPath = original })
	execLookPath = func(string) (string, error) { return "/trusted/habana-container-hook", nil }
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.DefaultConfig()
	spec := &specs.Spec{}
	if err := addPrestartHook(logger, spec, &cfg, []string{"0", "10"}); err != nil {
		t.Fatal(err)
	}
	if err := addCreateRuntimeHook(logger, spec, &cfg, []string{"10"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(spec.Hooks.Prestart[0].Args, []string{"/trusted/habana-container-hook", "--devices=0,10", "prestart"}) {
		t.Fatalf("prestart arguments = %v", spec.Hooks.Prestart[0].Args)
	}
	if !reflect.DeepEqual(spec.Hooks.CreateRuntime[0].Args, []string{"/trusted/habana-container-hook", "--devices=10", "createRuntime"}) {
		t.Fatalf("createRuntime arguments = %v", spec.Hooks.CreateRuntime[0].Args)
	}
	if err := addPrestartHook(logger, spec, &cfg, []string{"10"}); err != nil {
		t.Fatal(err)
	}
	if err := addCreateRuntimeHook(logger, spec, &cfg, []string{"0"}); err != nil {
		t.Fatal(err)
	}
	if len(spec.Hooks.Prestart) != 1 || spec.Hooks.Prestart[0].Args[1] != "--devices=10" ||
		len(spec.Hooks.CreateRuntime) != 1 || spec.Hooks.CreateRuntime[0].Args[1] != "--devices=0" {
		t.Fatal("existing hook kept stale device selection")
	}
}

func TestSaveSpecs(t *testing.T) {
	bundleFile := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(bundleFile, []byte(`{"ociVersion":"1.0.2","process":{"env":["LONG_VALUE=padding"]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	spec := &specs.Spec{Version: "1.0.2"}
	if err := saveSpecs(bundleFile, spec); err != nil {
		t.Fatal(err)
	}
	got, err := loadSpecs(bundleFile)
	if err != nil || !reflect.DeepEqual(got, spec) {
		t.Fatalf("saved spec = %v, %v", got, err)
	}
	info, err := os.Stat(bundleFile)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("spec permissions were not preserved: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(bundleFile))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary spec files remain: %v, %v", entries, err)
	}
	if err := saveSpecs(filepath.Join(t.TempDir(), "missing.json"), spec); err == nil {
		t.Fatal("missing spec file was accepted")
	}
}

func TestHookBinaryPath(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.Config
		execFn   func(string) (string, error)
		osexecFn func() (string, error)
		statFn   func(string) (fs.FileInfo, error)
		expPath  string
	}{
		{
			name: "binary found in path",
			cfg:  config.Config{},
			execFn: func(s string) (string, error) {
				return "/usr/bin/habana-container-hook", nil
			},
			osexecFn: nil,
			statFn:   nil,
			expPath:  "/usr/bin/habana-container-hook",
		},
		{
			name: "binary not found in path, but found in the (current) binary habana-container-runtime's dir",
			cfg:  config.Config{},
			execFn: func(s string) (string, error) {
				return "", fs.ErrNotExist
			},
			osexecFn: func() (string, error) {
				return "/os/exec/bin/habana-container-runtime", nil
			},
			statFn: func(s string) (fs.FileInfo, error) {
				return nil, nil
			},
			expPath: "/os/exec/bin/habana-container-hook",
		},
		{
			name: "binary not found in path, but found in binaries-dir",
			cfg: config.Config{
				BinariesDir: "/usr/local/bin/", // Same as configured in OpenShift
			},
			execFn: func(s string) (string, error) {
				return "", fs.ErrNotExist
			},
			osexecFn: func() (string, error) {
				return "", errors.New("Dummy Error")
			},
			statFn: func(s string) (fs.FileInfo, error) {
				return nil, nil
			},
			expPath: "/usr/local/bin/habana-container-hook",
		},
		{
			name: "binary not found in any path, trying the default",
			cfg: config.Config{
				BinariesDir: "/usr/local/bin/", // Same as configured in OpenShift
			},
			execFn: func(s string) (string, error) {
				return "", fs.ErrNotExist
			},
			osexecFn: func() (string, error) {
				return "", errors.New("Dummy Error")
			},
			statFn: func(s string) (fs.FileInfo, error) {
				if s == "/usr/local/bin/habana-container-hook" {
					return nil, fs.ErrNotExist
				}
				return nil, nil
			},
			expPath: "/usr/bin/habana-container-hook",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				execLookPath = exec.LookPath
				osExecutable = os.Executable
				osStat = os.Stat
			})
			execLookPath = tt.execFn
			osExecutable = tt.osexecFn
			osStat = tt.statFn
			got, err := hookBinaryPath(&tt.cfg)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.expPath {
				t.Errorf("got path %q, want %q", got, tt.expPath)
			}
		})
	}
	t.Run("returns errors when binary not found", func(t *testing.T) {
		cfg := config.Config{BinariesDir: "/some/dummy"}
		path, err := hookBinaryPath(&cfg)
		if err == nil {
			t.Error("want an error, got none with path", path)
		}
	})
}
