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
	"slices"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

func TestAllocatedDevices(t *testing.T) {
	major, minor := int64(261), int64(10)
	inventory := []*DevInfo{
		{Path: "/dev/accel/accel10", Major: 261, Minor: 10, Mode: unix.S_IFCHR},
		{Path: "/dev/accel/accel_controlD10", Major: 261, Minor: 138, Mode: unix.S_IFCHR},
	}
	tests := []struct {
		name    string
		device  specs.LinuxDevice
		rules   []specs.LinuxDeviceCgroup
		allowed bool
	}{
		{"allocated", specs.LinuxDevice{Type: "c", Major: major, Minor: minor}, []specs.LinuxDeviceCgroup{{Allow: true, Type: "c", Major: &major, Minor: &minor, Access: "rw"}}, true},
		{"no rule", specs.LinuxDevice{Type: "c", Major: major, Minor: minor}, nil, false},
		{"read only", specs.LinuxDevice{Type: "c", Major: major, Minor: minor}, []specs.LinuxDeviceCgroup{{Allow: true, Type: "c", Access: "r"}}, false},
		{"deny overrides allow", specs.LinuxDevice{Type: "c", Major: major, Minor: minor}, []specs.LinuxDeviceCgroup{{Allow: true, Type: "a", Access: "rwm"}, {Type: "c", Major: &major, Minor: &minor, Access: "w"}}, false},
		{"allow overrides deny", specs.LinuxDevice{Type: "c", Major: major, Minor: minor}, []specs.LinuxDeviceCgroup{{Type: "a", Access: "rwm"}, {Allow: true, Type: "c", Major: &major, Minor: &minor, Access: "rw"}}, true},
		{"wrong device number", specs.LinuxDevice{Path: "/dev/accel/accel10", Type: "c", Major: major, Minor: 0}, []specs.LinuxDeviceCgroup{{Allow: true, Type: "a", Access: "rwm"}}, false},
		{"block device", specs.LinuxDevice{Type: "b", Major: major, Minor: minor}, []specs.LinuxDeviceCgroup{{Allow: true, Type: "a", Access: "rwm"}}, false},
		{"control node alone", specs.LinuxDevice{Type: "c", Major: major, Minor: 138}, []specs.LinuxDeviceCgroup{{Allow: true, Type: "a", Access: "rwm"}}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := &specs.Spec{Linux: &specs.Linux{Devices: []specs.LinuxDevice{test.device}, Resources: &specs.LinuxResources{Devices: test.rules}}}
			got := AllocatedDevices(spec, inventory)
			if test.allowed && !slices.Equal(got, []string{"10"}) || !test.allowed && len(got) != 0 {
				t.Fatalf("allocated devices = %v, allowed = %v", got, test.allowed)
			}
		})
	}
	if got := AllocatedDevices(&specs.Spec{}, inventory); len(got) != 0 {
		t.Fatalf("missing Linux spec authorized %v", got)
	}
}

func TestSelectDevices(t *testing.T) {
	tests := []struct {
		name      string
		env       []string
		always    bool
		want      []string
		wantError bool
	}{
		{name: "all is bounded", env: []string{"HABANA_VISIBLE_DEVICES=all"}, want: []string{"10"}},
		{name: "numeric ID", env: []string{"HABANA_VISIBLE_DEVICES=10"}, want: []string{"10"}},
		{name: "outside allocation", env: []string{"HABANA_VISIBLE_DEVICES=0"}, wantError: true},
		{name: "partial allocation", env: []string{"HABANA_VISIBLE_DEVICES=10,0"}, wantError: true},
		{name: "empty", env: []string{"HABANA_VISIBLE_DEVICES="}},
		{name: "none", env: []string{"HABANA_VISIBLE_DEVICES=none"}},
		{name: "void", env: []string{"HABANA_VISIBLE_DEVICES=void"}},
		{name: "missing"},
		{name: "always mount is bounded", always: true, want: []string{"10"}},
		{name: "empty overrides always mount", env: []string{"HABANA_VISIBLE_DEVICES="}, always: true},
		{name: "prefix lookalike", env: []string{"HABANA_VISIBLE_DEVICES_OTHER=all"}},
		{name: "missing equals", env: []string{"HABANA_VISIBLE_DEVICES"}, wantError: true},
		{name: "duplicate key", env: []string{"HABANA_VISIBLE_DEVICES=10", "HABANA_VISIBLE_DEVICES=all"}, wantError: true},
		{name: "conflicting keys", env: []string{"HABANA_VISIBLE_DEVICES=10", "HABANA_VISIBLE_MODULES=none"}, wantError: true},
		{name: "consistent keys", env: []string{"HABANA_VISIBLE_DEVICES=10", "HABANA_VISIBLE_MODULES=all"}, want: []string{"10"}},
		{name: "empty list item", env: []string{"HABANA_VISIBLE_DEVICES=10,"}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := SelectDevices(test.env, []string{"10"}, test.always)
			if (err != nil) != test.wantError || !slices.Equal(got, test.want) {
				t.Fatalf("SelectDevices() = %v, %v; want %v, error=%v", got, err, test.want, test.wantError)
			}
		})
	}
}

func TestDevicesIDs(t *testing.T) {
	got := DevicesIDs([]string{"/dev/accel/accel10", "/dev/accel/accel_controlD10", "/dev/accel/accel0", "/dev/accel/accel0oops"})
	if !slices.Equal(got, []string{"10", "0"}) {
		t.Fatalf("DevicesIDs() = %v", got)
	}
}

func TestDeviceIdentitySelection(t *testing.T) {
	original := SysClassAccel
	t.Cleanup(func() { SysClassAccel = original })
	root := t.TempDir()
	SysClassAccel = filepath.Join(root, "accel")
	if err := os.MkdirAll(SysClassAccel+"10/device", 0755); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"module_id": "3", "pci_addr": "0000:10:00.0"} {
		if err := os.WriteFile(filepath.Join(SysClassAccel+"10/device", name), []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, env := range [][]string{
		{"HABANA_VISIBLE_MODULES=3"},
		{"HABANA_VISIBLE_PCI_BUS_IDS=0000:10:00.0"},
		{"HABANA_VISIBLE_DEVICES=10", "HABANA_VISIBLE_MODULES=3", "HABANA_VISIBLE_PCI_BUS_IDS=0000:10:00.0"},
	} {
		got, err := SelectDevices(env, []string{"10"}, false)
		if err != nil || !slices.Equal(got, []string{"10"}) {
			t.Fatalf("identity selection %v = %v, %v", env, got, err)
		}
	}
	var infiniband []string
	for _, name := range []string{"hbl_0", "hbl_10"} {
		device := filepath.Join(root, name)
		pci := "0000:10:00.0"
		if name == "hbl_10" {
			pci = "0000:11:00.0"
		}
		pciPath := filepath.Join(root, pci)
		if err := os.MkdirAll(pciPath, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(device, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(pciPath, filepath.Join(device, "device")); err != nil {
			t.Fatal(err)
		}
		infiniband = append(infiniband, device)
	}
	got, err := InfinibandForAccelerators([]string{"10"}, infiniband)
	if err != nil || !slices.Equal(got, infiniband[:1]) {
		t.Fatalf("PCI-associated interfaces = %v, %v", got, err)
	}
	got, err = InfinibandForAccelerators(nil, infiniband)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty selection exposed interfaces = %v, %v", got, err)
	}
}

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
