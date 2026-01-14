package crio

import (
	"os"
	"reflect"
	"testing"

	"github.com/HabanaAI/habana-container-runtime/internal/utils"
	"github.com/sirupsen/logrus"
)

const (
	testConfig string = `
		[crio.runtime]
		cdi_spec_dirs = [
			"/etc/cdi",
			"/var/run/cdi",
		]
		[crio.runtime.runtimes]
		`
	habanRuntimeConfiguration string = `
		[crio.runtime.runtimes.habana]
		runtime_path = "/path/to/runtime/habana-container-runtime"
		monitor_path = "/usr/libexec/crio/conmon"
		monitor_env = [
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/path/to/runtime",
		]
	`
)

func TestWithLogger(t *testing.T) {
	logger := logrus.New()
	cfg, err := New(WithLogger(logger), FromString(""))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.logger != logger {
		t.Errorf("expected logger to be set")
	}
}

func TestAddRuntime(t *testing.T) {
	cfg, err := New(FromString(testConfig))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = cfg.AddRuntime("/path/to/runtime")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Verify the runtime was added correctly
	expectedConfig := testConfig + habanRuntimeConfiguration
	expected, err := utils.NewToml(utils.TomlFromByte[interface{}]([]byte(expectedConfig)))
	if err != nil {
		t.Errorf("failed to load expected config: %v", err)
	}
	expectedSerialized, err := expected.Serialize()
	if err != nil {
		t.Errorf("failed to serialize expected config: %v", err)
	}
	serialized, err := cfg.config.Serialize()
	if err != nil {
		t.Errorf("failed to serialize config: %v", err)
	}
	if string(serialized) != string(expectedSerialized) {
		t.Errorf("expected serialized config to be:\n%s\nbut got:\n%s", string(expectedSerialized), string(serialized))
	}
}

func TestEnableCDI(t *testing.T) {
	cfg, err := New(FromString(""))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = cfg.EnableCDI()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	// Verify CDI was enabled correctly
	expectedConfig := `
		[crio]
		[crio.runtime]
		cdi_spec_dirs = [
			"/etc/cdi",
			"/var/run/cdi",
		]
	`
	expected, err := utils.NewToml(utils.TomlFromByte[map[string]interface{}]([]byte(expectedConfig)))
	if err != nil {
		t.Errorf("failed to load expected config: %v", err)
	}
	serialized, err := cfg.config.Serialize()
	if err != nil {
		t.Errorf("failed to serialize config: %v", err)
	}
	expectedSerialized, err := expected.Serialize()
	if err != nil {
		t.Errorf("failed to serialize expected config: %v", err)
	}
	if string(serialized) != string(expectedSerialized) {
		t.Errorf("expected serialized config to be:\n%s\nbut got:\n%s", string(expectedSerialized), string(serialized))
	}
}

func TestRemoveRuntime(t *testing.T) {
	config := testConfig + habanRuntimeConfiguration
	cfg, err := New(FromString(config))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = cfg.RemoveRuntime()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Verify the runtime was removed
	expected, err := utils.NewToml(utils.TomlFromByte[map[string]interface{}]([]byte(testConfig)))
	if err != nil {
		t.Errorf("failed to load expected config: %v", err)
	}
	if !reflect.DeepEqual(cfg.config.Data, expected.Data) {
		t.Errorf("expected config to be:\n%v\nbut got:\n%v", expected.Data, cfg.config.Data)
	}
}

func TestSave(t *testing.T) {
	savePath := "/tmp/containerd.toml"
	defer func() {
		// Clean up any test files created
		_ = os.Remove(savePath)
	}()
	cfg, err := New(
		WithLogger(logrus.New()),
		func(cc *CRIOConfig) error {
			cc.config = &utils.TomlConfig[map[string]interface{}]{Data: map[string]interface{}{}}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	err = cfg.Save(savePath)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		t.Errorf("expected file to be created at %s, but it does not exist", savePath)
	}
}

func TestIsModified(t *testing.T) {
	cfg, err := New(FromString(testConfig))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	modified, err := cfg.IsModified()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if modified {
		t.Errorf("expected config to be unmodified")
	}

	err = cfg.AddRuntime("/path/to/runtime")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	modified, err = cfg.IsModified()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !modified {
		t.Errorf("expected config to be modified after adding runtime")
	}

	serialziedData, err := cfg.Serialize()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	cfg, err = New(FromString(string(serialziedData)))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	err = cfg.RemoveRuntime()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	modified, err = cfg.IsModified()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !modified {
		t.Errorf("expected config to be modified after removing runtime")
	}
}
