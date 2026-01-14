package docker

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestNewDockerConfig(t *testing.T) {
	cfg, err := New()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil DockerConfig")
	}
}

func TestWithLogger(t *testing.T) {
	logger := logrus.New()
	cfg, err := New(WithLogger(logger))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.logger != logger {
		t.Errorf("expected logger to be set")
	}
}

func TestFromBytes(t *testing.T) {
	jsonData := []byte(`{"features":{},"runtimes":{}}`)
	cfg, err := New(FromBytes(jsonData))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.config == nil {
		t.Error("expected config to be loaded from bytes")
	}
	if len(*cfg.config) == 0 {
		t.Error("expected config to not be empty")
	}

	if _, ok := (*cfg.config)["features"]; !ok {
		t.Error("expected 'features' key in config")
	}

	if _, ok := (*cfg.config)["runtimes"]; !ok {
		t.Error("expected 'runtimes' key in config")
	}
}

func TestAddRuntime(t *testing.T) {
	jsonData := []byte(`{"features":{},"runtimes":{}}`)
	cfg, err := New(FromBytes(jsonData))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = cfg.AddRuntime("/bin")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len((*cfg.config)["runtimes"].(map[string]interface{})) == 0 {
		t.Error("expected runtime to be added")
	}

	if _, ok := (*cfg.config)["runtimes"].(map[string]interface{})["habana"]; !ok {
		t.Error("expected 'habana' runtime to be added")
	}
}

func TestEnableCDI(t *testing.T) {
	jsonData := []byte(`{"features":{},"runtimes":{}}`)
	cfg, err := New(FromBytes(jsonData))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = cfg.EnableCDI()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Verify CDI was enabled correctly
	if features, ok := (*cfg.config)["features"].(map[string]bool); !ok || !features["cdi"] {
		t.Error("expected CDI feature to be enabled")
	}
}

func TestRemoveRuntime(t *testing.T) {
	jsonData := []byte(`{"features":{},"runtimes":{"habana":{}, "other":{}}}`)
	cfg, err := New(FromBytes(jsonData))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = cfg.RemoveRuntime()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	runtimes, ok := (*cfg.config)["runtimes"].(map[string]interface{})
	if !ok {
		t.Error("expected 'runtimes' key to exist and be a map")
	} else if _, exists := runtimes["habana"]; exists {
		t.Error("expected 'habana' runtime to be removed")
	}
}

func TestRemoveRuntimeOnlyHabanaExists(t *testing.T) {
	jsonData := []byte(`{"features":{},"runtimes":{"habana":{}}}`)
	cfg, err := New(FromBytes(jsonData))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = cfg.RemoveRuntime()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	_, ok := (*cfg.config)["runtimes"].(map[string]interface{})
	if ok {
		t.Error("expected 'runtimes' key to be removed when only 'habana' exists")
	}
}

func TestSerialize(t *testing.T) {
	jsonData := []byte(`{"features":{},"runtimes":{}}`)
	cfg, err := New(FromBytes(jsonData))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	_, err = cfg.Serialize()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestSave(t *testing.T) {
	jsonData := []byte(`{"features":{},"runtimes":{}}`)
	cfg, err := New(FromBytes(jsonData))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	tmpFile := "test_docker_config.json"
	defer os.Remove(tmpFile)
	err = cfg.Save(tmpFile)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestIsModified(t *testing.T) {
	jsonData := []byte(`{"features":{},"runtimes":{}}`)
	cfg, err := New(FromBytes(jsonData))
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
	cfg, err = New(FromBytes(serialziedData))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	modified, err = cfg.IsModified()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if modified {
		t.Errorf("expected config to be unmodified")
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
