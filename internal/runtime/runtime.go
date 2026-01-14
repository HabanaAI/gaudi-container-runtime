package runtime

import (
	"fmt"

	"github.com/HabanaAI/habana-container-runtime/internal/runtime/containerd"
	"github.com/HabanaAI/habana-container-runtime/internal/runtime/crio"
	"github.com/HabanaAI/habana-container-runtime/internal/runtime/docker"
	"github.com/sirupsen/logrus"
)

type RuntimeBuilder interface {
	AddRuntime(string) error
	EnableCDI() error
	IsModified() (bool, error)
	RemoveRuntime() error
	RestartRuntime() error
	Save(string) error
	Serialize() ([]byte, error)
}

func NewRuntimeBuilder(runtimeType string, logger *logrus.Logger, configFilePath string) (RuntimeBuilder, error) {
	var builder RuntimeBuilder
	var err error

	switch runtimeType {
	case "containerd":
		builder, err = containerd.New(
			containerd.WithLogger(logger),
			containerd.FromConfigPath(configFilePath),
		)
	case "crio":
		builder, err = crio.New(
			crio.WithLogger(logger),
			crio.FromConfigPath(configFilePath),
		)
	case "docker":
		builder, err = docker.New(
			docker.WithLogger(logger),
			docker.FromConfigPath(configFilePath),
		)
	default:
		err = fmt.Errorf("unrecognized runtime '%v'", runtimeType)
	}
	if err != nil || builder == nil {
		return nil, fmt.Errorf("unable to load config for runtime %v: %v", runtimeType, err)
	}

	return builder, nil
}
