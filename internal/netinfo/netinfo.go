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
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// hlsNumInterfaceByType hold the known number of network ports (internal+external)
// for each Gaudi device we have
var hlsNumInterfaceByType = map[string]int{
	"gaudi":   10,
	"gaudi2":  24,
	"gaudi2b": 24,
	"gaudi2c": 24,
	"gaudi2d": 24,
	"gaudi2e": 24,
	"gaudi3":  24,
	"gaudi3d": 24,
}

type MACInfo struct {
	PciID       string   `json:"PCI_ID"`
	MACAddrList []string `json:"MAC_ADDR_LIST"`
}

type NetJSON struct {
	Info []MACInfo `json:"MAC_ADDR_INFO"`
}

func getDeviceTypeInterfacesCount(devType string) int {
	if val, exists := hlsNumInterfaceByType[devType]; exists {
		return val
	}
	return 24 // Default to 24 interfaces if device type is unknown
}

// Generates creates the mac address information for the requested accelerator devices,
// and saves the file in the default location '/etc/habanalabs/macAddrInfo.json'.
func Generate(devicesIDs []string, containerRootFS string) error {
	dir, err := openNetinfoDir(containerRootFS, "macAddrInfo.json", 0750)
	if err != nil {
		return err
	}
	defer dir.Close()

	return collect(dir, devicesIDs)
}

func symlinkCheck(filePath string) error {
	// First, ensure no parent directory in the path is a symlink.
	cleanPath := filepath.Clean(filePath)
	dir := filepath.Dir(cleanPath)
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root or cannot walk further.
			break
		}

		fi, err := os.Lstat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				// Parent doesn't exist yet; nothing to check at this level.
				dir = parent
				continue
			}
			return err
		}

		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path component %s is a symlink, which is not allowed", dir)
		}

		dir = parent
	}

	// Now check the file itself (if it exists) without following symlinks.
	fi, err := os.Lstat(filePath)
	if os.IsNotExist(err) {
		// Non-existent file is allowed; it will be created later.
		return nil
	} else if err != nil {
		return err
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("the file %s is a symlink, which is not allowed", filePath)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("the file %s is not a regular file", filePath)
	}
	return nil
}

func openNetinfoDir(containerRootFS, name string, mode uint32) (*os.File, error) {
	if err := symlinkCheck(path.Join(containerRootFS, "etc", "habanalabs", name)); err != nil {
		return nil, err
	}

	dir, err := os.OpenFile(containerRootFS, os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	for _, component := range []string{"etc", "habanalabs"} {
		flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC
		fd, err := unix.Openat(int(dir.Fd()), component, flags, 0)
		if errors.Is(err, unix.ENOENT) {
			if err := unix.Mkdirat(int(dir.Fd()), component, mode); err != nil && !errors.Is(err, unix.EEXIST) {
				dir.Close()
				return nil, err
			}
			fd, err = unix.Openat(int(dir.Fd()), component, flags, 0)
		}
		dir.Close()
		if err != nil {
			return nil, err
		}
		dir = os.NewFile(uintptr(fd), component)
	}
	return dir, nil
}

func writeNetinfoFile(dir *os.File, name string, write func(io.Writer) error) error {
	dirFD := int(dir.Fd())
	var stat unix.Stat_t
	err := unix.Fstatat(dirFD, name, &stat, unix.AT_SYMLINK_NOFOLLOW)
	if err == nil {
		if stat.Mode&unix.S_IFMT != unix.S_IFREG {
			return fmt.Errorf("the file %s is not a regular file", name)
		}
	} else if !errors.Is(err, unix.ENOENT) {
		return err
	}

	tempName := "." + name + "-" + rand.Text()
	fd, err := unix.Openat(dirFD, tempName, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0644)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), tempName)
	defer file.Close()
	defer unix.Unlinkat(dirFD, tempName, 0)

	if err := write(file); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return unix.Renameat(dirFD, tempName, dirFD, name)
}

// collect gathers MAC address info about the requested accelerators IDs.
func collect(dir *os.File, devicesIDs []string) error {
	netData, err := netConfig(devicesIDs)
	if err != nil {
		return err
	}

	if len(netData.Info) > 0 {

		// Creating the file empty causes issues in hcl library. So we must
		// verify it won't be created if there's no data at all. The other option
		// is to insert logic to parse the expected internal and external ports from
		// a mask, and create the config accordinly. Only than the file info will be correct,
		// otherwise we better error out when ext ports are disabled. related issue: SW-190964.
		return writeNetinfoFile(dir, "macAddrInfo.json", func(writer io.Writer) error {
			encoder := json.NewEncoder(writer)
			encoder.SetIndent("", "  ")
			return encoder.Encode(netData)
		})
	}

	return nil
}

func GaudinetFile(logger *slog.Logger, containerRootFS, source string) error {
	info, err := os.Stat(source)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			logger.Info(fmt.Sprintf("file does not exist on host: %s", source))
			return nil
		}
		return err
	}

	// Skip copying an empty file to avoid HCL problem
	if info.Size() == 0 {
		logger.Info("File exists but it's empty, skiping...")
		return nil
	}

	srcFile, err := os.Open(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer srcFile.Close()

	dir, err := openNetinfoDir(containerRootFS, "gaudinet.json", 0755)
	if err != nil {
		return err
	}
	defer dir.Close()

	return writeNetinfoFile(dir, "gaudinet.json", func(writer io.Writer) error {
		_, err := io.Copy(writer, srcFile)
		return err
	})
}

func netConfig(devices []string) (NetJSON, error) {
	if len(devices) == 0 {
		return NetJSON{}, nil
	}

	deviceType, err := deviceType(os.DirFS("/"), devices[0])
	if err != nil {
		return NetJSON{}, fmt.Errorf("netConfig: %w", err)
	}

	devicesPCI, err := devicesPCIAddresses(os.DirFS("/"), devices)
	if err != nil {
		return NetJSON{}, err
	}

	netInfo, err := devicesMACAddress(os.DirFS("/"), devicesPCI, deviceType)
	if err != nil {
		return NetJSON{}, err
	}

	return NetJSON{Info: netInfo}, nil
}

func deviceType(fsys fs.FS, deviceID string) (string, error) {
	devTypePath := fmt.Sprintf("sys/class/accel/accel%s/device/device_type", deviceID)

	content, err := fs.ReadFile(fsys, devTypePath)
	if err != nil {
		return "", fmt.Errorf("deviceType: %w", err)
	}
	data := strings.TrimSpace(string(content))

	parts := strings.Fields(data)
	if len(parts) == 0 {
		return "", fmt.Errorf("deviceType info not found")
	}

	return strings.ToLower(parts[0]), nil
}

// returns a map of requested Habana devices in the form of map[hlID]pciAddress
func devicesPCIAddresses(fsys fs.FS, devices []string) (map[string]string, error) {
	pciInfo := make(map[string]string)

	for _, devID := range devices {
		devName := "accel" + devID
		content, err := fs.ReadFile(fsys, path.Clean(path.Join("sys/class/accel", devName, "device", "pci_addr")))
		if err != nil {
			return nil, err
		}
		pciAddr := strings.TrimSpace(string(content))
		pciInfo[devID] = pciAddr
	}

	return pciInfo, nil
}

func devicesMACAddress(fsys fs.FS, pciDevices map[string]string, devType string) ([]MACInfo, error) {
	var devInfo []MACInfo

	// Collect external ports mac addresses
	extPorts, err := extPortsMACAddress(fsys, pciDevices)
	if err != nil {
		return nil, err
	}

	// Fill MAC addresses data based on port type external or internal
	for hlID, pciID := range pciDevices {
		var macAddressList []string

		for i := 0; i < getDeviceTypeInterfacesCount(devType); i++ {
			// If the port is recognized as external, we add the readl mac addresss,
			// otherwise, we add a broadcast mac address for each internal port
			if _, exists := extPorts[hlID][i]; exists {
				macAddressList = append(macAddressList, extPorts[hlID][i])
			} else {
				macAddressList = append(macAddressList, "ff:ff:ff:ff:ff:ff")
			}
		}

		devInfo = append(devInfo, MACInfo{
			PciID:       pciID,
			MACAddrList: macAddressList,
		})
	}

	return devInfo, nil
}

// getExtPorts receives Habana list of devices, and returns their MacAddress and device port
// of their external interfaces. Returns map[hlID]map[devPort]PciAddr
func extPortsMACAddress(fsys fs.FS, pciDevices map[string]string) (map[string]map[int]string, error) {
	extInfo := make(map[string]map[int]string)

	for hlID, pci := range pciDevices {
		netPath := fmt.Sprintf("sys/bus/pci/devices/%s/net", pci)
		// check if path exists
		if _, err := fs.Stat(fsys, netPath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue // No network interfaces for this device
			}
			return nil, fmt.Errorf("extPortsMACAddress: %w", err)
		}

		ifaces, err := fs.ReadDir(fsys, netPath)
		if err != nil {
			return nil, err
		}

		netinfo := make(map[int]string)
		for _, inet := range ifaces {
			// Get MAC Address
			mac, err := fs.ReadFile(fsys, path.Clean(path.Join(netPath, inet.Name(), "address")))
			if err != nil {
				return nil, err
			}

			// Get dev port
			devPort, err := fs.ReadFile(fsys, path.Clean(path.Join(netPath, inet.Name(), "dev_port")))
			if err != nil {
				return nil, err
			}
			devPortInt, err := strconv.Atoi(strings.TrimSpace(string(devPort)))
			if err != nil {
				return nil, err
			}
			macAddr := strings.TrimSpace(string(mac))
			netinfo[devPortInt] = macAddr
		}
		extInfo[hlID] = netinfo

	}
	return extInfo, nil
}
