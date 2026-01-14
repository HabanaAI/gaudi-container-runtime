/*
 * Copyright (c) 2025, HabanaLabs Ltd.  All rights reserved.
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
package discover

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasExternalInterfaces(t *testing.T) {
	type args struct {
		deviceIds []string
		tempFiles map[string]string
		emptyDirs []string
	}
	tests := []struct {
		name       string
		args       args
		shouldHave bool
	}{
		{
			name: "no device ids",
			args: args{
				deviceIds: []string{},
				emptyDirs: []string{"sys/bus/pci/devices/0000:10:00.0/net"},
				tempFiles: map[string]string{
					"sys/class/accel/accel1/device/pci_addr": "0000:10:00.0",
				},
			},
			shouldHave: false,
		},
		{
			name: "external interface single device",
			args: args{
				deviceIds: []string{"1"},
				emptyDirs: []string{"sys/bus/pci/devices/0000:10:00.0/net"},
				tempFiles: map[string]string{
					"sys/class/accel/accel1/device/pci_addr": "0000:10:00.0",
				},
			},
			shouldHave: true,
		},
		{
			name: "external interface",
			args: args{
				deviceIds: []string{"0", "1"},
				emptyDirs: []string{"sys/bus/pci/devices/0000:09:00.0/net", "sys/bus/pci/devices/0000:10:00.0/net"},
				tempFiles: map[string]string{
					"sys/class/accel/accel0/device/pci_addr": "0000:09:00.0\n",
					"sys/class/accel/accel1/device/pci_addr": "0000:10:00.0\n",
				},
			},
			shouldHave: true,
		},
		{
			name: "external interface with controlD",
			args: args{
				deviceIds: []string{"0", "1"},
				emptyDirs: []string{"sys/bus/pci/devices/0000:09:00.0/net", "sys/bus/pci/devices/0000:10:00.0/net"},
				tempFiles: map[string]string{
					"sys/class/accel/accel0/device/pci_addr": "0000:09:00.0",
					"sys/class/accel/accel1/device/pci_addr": "0000:10:00.0",
					"sys/class/accel/accel_controlD1/test":   "12345678",
				},
			},
			shouldHave: true,
		},
		{
			name: "no external interface",
			args: args{
				deviceIds: []string{"0", "1"},
				emptyDirs: []string{"sys/bus/pci/devices/0000:09:00.0", "sys/bus/pci/devices/0000:10:00.0"},
				tempFiles: map[string]string{
					"sys/class/accel/accel0/device/pci_addr": "0000:09:00.0",
					"sys/class/accel/accel1/device/pci_addr": "0000:10:00.0",
				},
			},
			shouldHave: false,
		},
		{
			name: "no external interface (partial)",
			args: args{
				deviceIds: []string{"0", "1"},
				emptyDirs: []string{"sys/bus/pci/devices/0000:09:00.0/net", "sys/bus/pci/devices/0000:10:00.0"},
				tempFiles: map[string]string{
					"sys/class/accel/accel0/device/pci_addr": "0000:09:00.0",
					"sys/class/accel/accel1/device/pci_addr": "0000:10:00.0",
				},
			},
			shouldHave: false,
		},
	}

	for _, tt := range tests {
		tpath, err := os.MkdirTemp("", "discovertest")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}

		defer os.RemoveAll(tpath)

		for _, path := range tt.args.emptyDirs {
			if err := os.MkdirAll(filepath.Join(tpath, path), 0755); err != nil {
				t.Fatalf("failed to create empty dir %s: %v", path, err)
			}
		}

		for path, content := range tt.args.tempFiles {
			full := filepath.Join(tpath, path)
			dir := filepath.Dir(full)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("failed to create dir %s: %v", dir, err)
			}

			if err := os.WriteFile(full, []byte(content), 0644); err != nil {
				t.Fatalf("failed writing temp file %s: %v", full, err)
			}
		}

		t.Run(tt.name, func(t *testing.T) {
			got := HasExternalInterfaces(tpath, tt.args.deviceIds)
			if got != tt.shouldHave {
				t.Errorf("%s - HasExternalInterfaces() = %v, want %v", tt.name, got, tt.shouldHave)
			}
		})
	}
}
