package discover

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

var selectionKeys = []string{"HABANA_VISIBLE_DEVICES", "HABANA_VISIBLE_MODULES", "HABANA_VISIBLE_PCI_BUS_IDS"}

func AcceleratorID(devicePath string) (string, bool) {
	name := filepath.Base(devicePath)
	for _, prefix := range []string{"accel_controlD", "accel"} {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		id := strings.TrimPrefix(name, prefix)
		number, err := strconv.ParseUint(id, 10, 32)
		return id, err == nil && strconv.FormatUint(number, 10) == id
	}
	return "", false
}

func AllocatedDevices(spec *specs.Spec, inventory []*DevInfo) []string {
	var allocated []string
	if spec.Linux == nil || spec.Linux.Resources == nil {
		return allocated
	}
	for _, host := range inventory {
		id, valid := AcceleratorID(host.Path)
		if !valid || filepath.Base(host.Path) != "accel"+id || host.Mode&unix.S_IFMT != unix.S_IFCHR {
			continue
		}
		for _, device := range spec.Linux.Devices {
			if device.Type != "c" || device.Major != int64(host.Major) || device.Minor != int64(host.Minor) {
				continue
			}
			permissions := make(map[rune]bool)
			for _, rule := range spec.Linux.Resources.Devices {
				if rule.Type != "a" && rule.Type != "c" && rule.Type != "" {
					continue
				}
				if rule.Major != nil && *rule.Major != device.Major || rule.Minor != nil && *rule.Minor != device.Minor {
					continue
				}
				for _, access := range rule.Access {
					permissions[access] = rule.Allow
				}
			}
			if permissions['r'] && permissions['w'] && !slices.Contains(allocated, id) {
				allocated = append(allocated, id)
			}
		}
	}
	return allocated
}

func ResolveDevices(spec *specs.Spec, allowEnvironment, alwaysMount bool) ([]string, error) {
	if spec.Process == nil {
		return nil, fmt.Errorf("OCI process is missing")
	}
	if !allowEnvironment && (spec.Linux == nil || len(spec.Linux.Devices) == 0) {
		return nil, nil
	}
	var inventory []*DevInfo
	for _, devicePath := range AcceleratorDevices() {
		device, err := DeviceInfo(devicePath)
		if err != nil {
			return nil, err
		}
		inventory = append(inventory, device)
	}
	authorized := AllocatedDevices(spec, inventory)
	if allowEnvironment {
		var paths []string
		for _, device := range inventory {
			if device.Mode&unix.S_IFMT == unix.S_IFCHR {
				paths = append(paths, device.Path)
			}
		}
		authorized = DevicesIDs(paths)
	}
	if len(authorized) == 0 {
		return nil, nil
	}
	return SelectDevices(spec.Process.Env, authorized, alwaysMount)
}

func SelectDevices(env, authorized []string, alwaysMount bool) ([]string, error) {
	var selected []string
	seen := make(map[string]bool)
	for _, entry := range env {
		key, value, found := strings.Cut(entry, "=")
		if !slices.Contains(selectionKeys, key) {
			continue
		}
		if !found || seen[key] {
			return nil, fmt.Errorf("invalid or duplicate device selector %s", key)
		}
		seen[key] = true
		var current []string
		switch value {
		case "all":
			current = append(current, authorized...)
		case "", "none", "void":
		default:
			for _, requested := range strings.Split(value, ",") {
				matched := false
				for _, id := range authorized {
					identity := id
					var err error
					switch key {
					case "HABANA_VISIBLE_MODULES":
						identity, err = AcceleratorModuleID(id)
					case "HABANA_VISIBLE_PCI_BUS_IDS":
						identity, err = AcceleratorPciID(id)
					}
					if err != nil {
						return nil, err
					}
					if requested != "" && requested == identity {
						matched = true
						if !slices.Contains(current, id) {
							current = append(current, id)
						}
					}
				}
				if !matched {
					return nil, fmt.Errorf("%s requests an unallocated or unknown device %q", key, requested)
				}
			}
		}
		slices.Sort(current)
		if len(seen) > 1 && !slices.Equal(selected, current) {
			return nil, fmt.Errorf("conflicting Habana device selectors")
		}
		selected = current
	}
	if len(seen) == 0 && alwaysMount {
		return append([]string(nil), authorized...), nil
	}
	return selected, nil
}

func InfinibandForAccelerators(deviceIDs, infinibandDevices []string) ([]string, error) {
	var selected []string
	for _, id := range deviceIDs {
		pciID, err := AcceleratorPciID(id)
		if err != nil {
			return nil, err
		}
		if pciID == "" {
			return nil, fmt.Errorf("missing PCI identity for accelerator %s", id)
		}
		for _, device := range infinibandDevices {
			devicePath, err := filepath.EvalSymlinks(filepath.Join(device, "device"))
			if err != nil {
				return nil, fmt.Errorf("resolving infiniband device: %w", err)
			}
			for parent := devicePath; parent != filepath.Dir(parent); parent = filepath.Dir(parent) {
				if filepath.Base(parent) == pciID && !slices.Contains(selected, device) {
					selected = append(selected, device)
					break
				}
			}
		}
	}
	return selected, nil
}
