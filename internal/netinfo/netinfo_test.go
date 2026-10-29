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
	"errors"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"net"
	"os"
	"path"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sys/unix"
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

	fifo := path.Join(subdir, "fifo")
	if err := unix.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}

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
		{
			name:      "FIFO should return an error",
			filePath:  fifo,
			expectErr: true,
		},
		{
			name:      "directory should return an error",
			filePath:  subdir,
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

func TestSymlinkCheckCharacterDevice(t *testing.T) {
	info, err := os.Lstat("/dev/null")
	if err != nil {
		t.Skipf("character device unavailable: %v", err)
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		t.Skip("/dev/null is not a character device")
	}
	if err := symlinkCheck("/dev/null"); err == nil {
		t.Fatal("expected character device rejection")
	}
}

func TestNetinfoRejectsUnsafePaths(t *testing.T) {
	source := path.Join(t.TempDir(), "source.json")
	if err := os.WriteFile(source, []byte(`{"routes":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	writers := []struct {
		name  string
		write func(string) error
	}{
		{"macAddrInfo.json", func(rootfs string) error { return Generate(nil, rootfs) }},
		{"gaudinet.json", func(rootfs string) error { return GaudinetFile(logger, rootfs, source) }},
	}
	for _, writer := range writers {
		for _, kind := range []string{"FIFO", "socket", "directory", "symlink", "character device", "block device", "etc symlink", "habanalabs symlink"} {
			t.Run(writer.name+"/"+kind, func(t *testing.T) {
				rootfs, err := os.MkdirTemp("", "netinfo-")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { os.RemoveAll(rootfs) })
				outside := t.TempDir()
				dirPath := path.Join(rootfs, "etc", "habanalabs")
				if err := os.MkdirAll(dirPath, 0755); err != nil {
					t.Fatal(err)
				}
				destination := path.Join(dirPath, writer.name)
				switch kind {
				case "FIFO":
					err = unix.Mkfifo(destination, 0600)
				case "socket":
					var listener net.Listener
					listener, err = net.Listen("unix", destination)
					if err == nil {
						t.Cleanup(func() { listener.Close() })
					}
				case "directory":
					err = os.Mkdir(destination, 0755)
				case "symlink":
					err = os.Symlink(source, destination)
				case "character device", "block device":
					mode, device := uint32(unix.S_IFCHR|0600), unix.Mkdev(1, 7)
					if kind == "block device" {
						mode, device = unix.S_IFBLK|0600, unix.Mkdev(0, 0)
					}
					err = unix.Mknod(destination, mode, int(device))
					if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
						t.Skipf("device creation unavailable: %v", err)
					}
				case "etc symlink", "habanalabs symlink":
					if err := os.Remove(dirPath); err != nil {
						t.Fatal(err)
					}
					if kind == "etc symlink" {
						dirPath = path.Dir(dirPath)
						if err := os.Remove(dirPath); err != nil {
							t.Fatal(err)
						}
					}
					err = os.Symlink(outside, dirPath)
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := writer.write(rootfs); err == nil {
					t.Fatal("expected unsafe path rejection")
				}
				entries, err := os.ReadDir(outside)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 0 {
					t.Fatalf("created entries outside rootfs: %v", entries)
				}
			})
		}
	}
}

func TestWriteNetinfoFile(t *testing.T) {
	for _, name := range []string{"macAddrInfo.json", "gaudinet.json"} {
		for _, scenario := range []string{"new", "hard link", "write failure", "existing FIFO", "symlink race", "FIFO race", "parent race"} {
			t.Run(name+"/"+scenario, func(t *testing.T) {
				rootfs := t.TempDir()
				dir, err := openNetinfoDir(rootfs, name, 0755)
				if err != nil {
					t.Fatal(err)
				}
				defer dir.Close()
				dirPath := path.Join(rootfs, "etc", "habanalabs")
				destination := path.Join(dirPath, name)
				outside := t.TempDir()
				original := path.Join(outside, "original")
				if err := os.WriteFile(original, []byte("original contents"), 0600); err != nil {
					t.Fatal(err)
				}
				if scenario != "new" {
					if err := os.Link(original, destination); err != nil {
						t.Fatal(err)
					}
				}
				if scenario == "existing FIFO" {
					if err := os.Remove(destination); err != nil {
						t.Fatal(err)
					}
					if err := unix.Mkfifo(destination, 0600); err != nil {
						t.Fatal(err)
					}
				}
				writerCalled := false
				err = writeNetinfoFile(dir, name, func(writer io.Writer) error {
					writerCalled = true
					switch scenario {
					case "symlink race", "FIFO race":
						if err := os.Remove(destination); err != nil {
							t.Fatal(err)
						}
						if scenario == "symlink race" {
							if err := os.Symlink(original, destination); err != nil {
								t.Fatal(err)
							}
						} else if err := unix.Mkfifo(destination, 0600); err != nil {
							t.Fatal(err)
						}
					case "parent race":
						movedPath := path.Join(rootfs, "etc", "original")
						if err := os.Rename(dirPath, movedPath); err != nil {
							t.Fatal(err)
						}
						if err := os.Symlink(outside, dirPath); err != nil {
							t.Fatal(err)
						}
						dirPath = movedPath
						destination = path.Join(dirPath, name)
					}
					if _, err := io.WriteString(writer, "new contents"); err != nil {
						return err
					}
					if scenario == "write failure" {
						return io.ErrUnexpectedEOF
					}
					return nil
				})
				if scenario == "write failure" {
					if !errors.Is(err, io.ErrUnexpectedEOF) {
						t.Fatalf("expected write error, got %v", err)
					}
				} else if scenario == "existing FIFO" {
					if err == nil || writerCalled {
						t.Fatalf("expected rejection before writing, got %v", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				content, err := os.ReadFile(original)
				if err != nil || string(content) != "original contents" {
					t.Fatalf("outside file modified: %q, %v", content, err)
				}
				if _, err := os.Lstat(path.Join(outside, name)); !os.IsNotExist(err) {
					t.Fatalf("unexpected output outside rootfs: %v", err)
				}
				entries, err := os.ReadDir(dirPath)
				if err != nil || len(entries) != 1 || entries[0].Name() != name {
					t.Fatalf("unexpected output directory contents: %v, %v", entries, err)
				}
				info, err := os.Lstat(destination)
				if err != nil {
					t.Fatal(err)
				}
				if scenario == "existing FIFO" {
					if info.Mode()&os.ModeNamedPipe == 0 {
						t.Fatal("FIFO was modified")
					}
					return
				}
				if !info.Mode().IsRegular() {
					t.Fatalf("output is not regular: %v", info.Mode())
				}
				want := "new contents"
				if scenario == "write failure" {
					want = "original contents"
				}
				content, err = os.ReadFile(destination)
				if err != nil || string(content) != want {
					t.Fatalf("output contents: got %q, want %q, error %v", content, want, err)
				}
			})
		}
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
