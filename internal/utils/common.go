package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
)

type Serialize interface {
	Serialize() ([]byte, error)
}

func ComputeChecksum(data []byte) string {
	hasher := sha256.New()
	_, err := hasher.Write(data)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func RunCommandInHostNamespace(args []string) (*bytes.Buffer, error) {
	cmd := []string{"nsenter", "--target", "1", "--mount", "--uts", "--ipc", "--net", "--pid", "--"}
	cmd = append(cmd, args...)

	var out bytes.Buffer
	cm := exec.Command(cmd[0], cmd[1:]...)
	cm.Stdout = &out
	cm.Stderr = os.Stderr
	if err := cm.Run(); err != nil {
		return nil, fmt.Errorf("unable to restart containerd: %v", err)
	}

	return &out, nil
}

func GetProcessPid(processName string) (string, error) {
	out, err := RunCommandInHostNamespace([]string{"pgrep", processName})
	if err != nil {
		return "", fmt.Errorf("unable to get pid of %s: %v", processName, err)
	}

	pid := string(bytes.TrimSpace(out.Bytes()))
	if pid == "" {
		return "", fmt.Errorf("process %s not found", processName)
	}

	return pid, nil
}

func CreateFileIfNotExists(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("unable to create file %s: %v", filePath, err)
		}
		defer file.Close()
	}
	return nil
}
