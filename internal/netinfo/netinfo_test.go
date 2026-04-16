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
package netinfo

import (
	"io"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"path"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
)

func TestDeviceType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fsys     fstest.MapFS
		want     string
		expError bool
	}{
		{
			name:  "happy path",
			input: "0",
			fsys: fstest.MapFS{
				"sys/class/accel/accel0/device/device_type": &fstest.MapFile{
					Data: []byte("GAUDI2"),
				},
			},
			want:     "gaudi2",
			expError: false,
		},
		{
			name:  "Gaudi3D check",
			input: "0",
			fsys: fstest.MapFS{
				"sys/class/accel/accel0/device/device_type": &fstest.MapFile{
					Data: []byte("GAUDI3D"),
				},
			},
			want:     "gaudi3d",
			expError: false,
		},
		{
			name:  "file contains new line",
			input: "0",
			fsys: fstest.MapFS{
				"sys/class/accel/accel0/device/device_type": &fstest.MapFile{
					Data: []byte("GAUDI2\n"),
				},
			},
			want:     "gaudi2",
			expError: false,
		},
		{
			name:  "file contains new line and space char",
			input: "0",
			fsys: fstest.MapFS{
				"sys/class/accel/accel0/device/device_type": &fstest.MapFile{
					Data: []byte("GAUDI2\t\n"),
				},
			},
			want:     "gaudi2",
			expError: false,
		},
		{
			name:  "empty file returns an error",
			input: "0",
			fsys: fstest.MapFS{
				"sys/class/accel/accel0/device/device_type": &fstest.MapFile{
					Data: []byte(""),
				},
			},
			want:     "",
			expError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := deviceType(tt.fsys, tt.input)
			if tt.expError && err == nil {
				t.Fatal("expected and error, got none")
			}
			if !tt.expError && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDevicesPCIAddresses(t *testing.T) {
	fsys := fstest.MapFS{
		"sys/class/accel/accel0/device/pci_addr": &fstest.MapFile{
			Data: []byte("0000:09:00.0"),
		},
		"sys/class/accel/accel1/device/pci_addr": &fstest.MapFile{
			Data: []byte("0000:4a:00.0"),
		},
		"sys/class/accel/accel2/device/pci_addr": &fstest.MapFile{
			Data: []byte("0000:3a:00.0"),
		},
	}

	want := map[string]string{
		"0": "0000:09:00.0",
		"1": "0000:4a:00.0",
		"2": "0000:3a:00.0",
	}

	got, err := devicesPCIAddresses(fsys, []string{"0", "1", "2"})
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(got, want) {
		t.Error(cmp.Diff(got, want))
	}
}

func TestDevicesMACAddress(t *testing.T) {
	type args struct {
		fsys       fs.FS
		pciDevices map[string]string
		devType    string
	}
	tests := []struct {
		name string
		args args
		want []MACInfo
	}{
		{
			// enp51s0d22  enp51s0d23  enp51s0d8
			name: "happy path",
			args: args{
				fsys: fstest.MapFS{
					"sys/bus/pci/devices/0000:09:00.0/net/enp51s0d22/dev_port": &fstest.MapFile{
						Data: []byte("22"),
					},
					"sys/bus/pci/devices/0000:09:00.0/net/enp51s0d22/address": &fstest.MapFile{
						Data: []byte("b0:fd:0b:d6:15:9f"),
					},
					"sys/bus/pci/devices/0000:09:00.0/net/enp51s0d23/dev_port": &fstest.MapFile{
						Data: []byte("23"),
					},
					"sys/bus/pci/devices/0000:09:00.0/net/enp51s0d23/address": &fstest.MapFile{
						Data: []byte("b0:fd:0b:d6:15:a0"),
					},
					"sys/bus/pci/devices/0000:09:00.0/net/enp51s0d8/dev_port": &fstest.MapFile{
						Data: []byte("8"),
					},
					"sys/bus/pci/devices/0000:09:00.0/net/enp51s0d8/address": &fstest.MapFile{
						Data: []byte("b0:fd:0b:d6:15:91"),
					},
				},
				devType: "gaudi2",
				pciDevices: map[string]string{
					"0": "0000:09:00.0",
				},
			},
			want: []MACInfo{
				{
					PciID: "0000:09:00.0",
					MACAddrList: []string{
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"b0:fd:0b:d6:15:91",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"b0:fd:0b:d6:15:9f",
						"b0:fd:0b:d6:15:a0",
					},
				},
			},
		},
		{
			// enp51s0d22  enp51s0d23  enp51s0d8
			name: "no external interfaces path",
			args: args{
				fsys:    fstest.MapFS{},
				devType: "gaudi2",
				pciDevices: map[string]string{
					"0": "0000:09:00.0",
				},
			},
			want: []MACInfo{
				{
					PciID: "0000:09:00.0",
					MACAddrList: []string{
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
					},
				},
			},
		},
		{
			// enp51s0d22  enp51s0d23  enp51s0d8
			name: "unknown device type",
			args: args{
				fsys:    fstest.MapFS{},
				devType: "xyz",
				pciDevices: map[string]string{
					"0": "0000:09:00.0",
				},
			},
			want: []MACInfo{
				{
					PciID: "0000:09:00.0",
					MACAddrList: []string{
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
						"ff:ff:ff:ff:ff:ff",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := devicesMACAddress(tt.args.fsys, tt.args.pciDevices, tt.args.devType)
			if err != nil {
				t.Fatal(err)
			}
			if !cmp.Equal(got, tt.want) {
				t.Error(cmp.Diff(got, tt.want))
			}
		})
	}
}

func TestGaudinetFile(t *testing.T) {
	t.Run("copies file when there is value", gaudinetCopyFile)
	t.Run("ignores file exists but empty", gaudinetIgnoreEmptyFile)
	t.Run("ignore file does not exist", gaudinetIgnoreNonExistentFile)
}

func TestSymlinkCheck(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gnet-*")
	if err != nil {
		t.Fatal(err)
	}
	subdir, err := os.MkdirTemp(tmpDir, "subdir-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})

	// Create a regular file
	regFile := path.Join(subdir, "regular-file")
	err = os.WriteFile(regFile, []byte("regular file content"), 0644)
	if err != nil {
		t.Fatalf("failed writing temp regular file: %v", err)
	}

	// Create a symlink to the regular file
	symlink := path.Join(subdir, "symlink-file")
	err = os.Symlink(regFile, symlink)
	if err != nil {
		t.Fatalf("failed creating symlink: %v", err)
	}

	// Create a dangling symlink (pointing to a non-existent target)
	danglingSymlink := path.Join(subdir, "dangling-symlink")
	err = os.Symlink(path.Join(subdir, "non-existent-target"), danglingSymlink)
	if err != nil {
		t.Fatalf("failed creating dangling symlink: %v", err)
	}

	symlinkDir := path.Join(tmpDir, "symlink-dir")
	err = os.Symlink(subdir, symlinkDir)
	if err != nil {
		t.Fatalf("failed creating symlink: %v", err)
	}
	symlinkDirFile := path.Join(symlinkDir, "regular-file")

	tests := []struct {
		name      string
		filePath  string
		expectErr bool
	}{
		{
			name:      "regular file should pass",
			filePath:  regFile,
			expectErr: false,
		},
		{
			name:      "symlink should return an error",
			filePath:  symlink,
			expectErr: true,
		},
		{
			name:      "dangling symlink should return an error",
			filePath:  danglingSymlink,
			expectErr: true,
		},
		{
			name:      "symlink to directory should return an error",
			filePath:  symlinkDirFile,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := symlinkCheck(tt.filePath)
			if tt.expectErr && err == nil {
				t.Fatal("expected an error, got none")
			}
			if !tt.expectErr && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

func gaudinetCopyFile(t *testing.T) {
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})
	logger := slog.New(h)
	tmpDir, err := os.MkdirTemp("", "gnet-*")
	if err != nil {
		t.Fatal(err)
	}

	msg := "some cool data"
	// Source file
	f, err := os.CreateTemp(tmpDir, "gaudinet-*")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(msg)

	t.Cleanup(func() {
		_ = f.Close()
		_ = os.RemoveAll(tmpDir)
	})

	log.Println("temp file:", f.Name())
	err = GaudinetFile(logger, tmpDir, f.Name())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Compare original file and destination file
	content, err := os.ReadFile(path.Join(tmpDir, "etc", "habanalabs", "gaudinet.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != msg {
		t.Errorf("expect gaudinet content %q, got %q", string(content), msg)
	}
}

func gaudinetIgnoreEmptyFile(t *testing.T) {
	h := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})
	logger := slog.New(h)
	d, err := os.MkdirTemp("", "gnet-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(d)
	})

	// Create an empty file
	err = os.WriteFile(d+"/empty-gaudinet-file", []byte{}, 0744)
	if err != nil {
		t.Fatalf("failed writing temp empty file: %v", err)
	}

	err = GaudinetFile(logger, d, "empty-gaudinet-file")
	if err != nil {
		t.Error(err)
	}
}

func gaudinetIgnoreNonExistentFile(t *testing.T) {
	h := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})
	logger := slog.New(h)

	d, err := os.MkdirTemp("", "gnet-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(d)
	})

	err = GaudinetFile(logger, d, "not-exist")
	if err != nil {
		t.Fatalf("did not expect and error, got %v", err)
	}
}
