package utils

import (
	"reflect"
	"testing"
)

func TestTomlSetPathFromEmptyBytes(t *testing.T) {
	config, err := NewToml(
		TomlFromByte[map[string]interface{}]([]byte{}),
	)
	if err != nil {
		t.Fatalf("failed to create TomlConfig: %v", err)
	}
	err = config.SetPath([]string{"a", "b", "c"}, 1)
	if err != nil {
		t.Fatalf("SetPath failed: %v", err)
	}
	expected := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": 1,
			},
		},
	}
	if !reflect.DeepEqual(config.Data, expected) {
		t.Fatalf("SetPath result mismatch: got %v, want %v", config.Data, expected)
	}
}

func TestTomlSetPathForStructFromEmptyBytes(t *testing.T) {
	type Inner struct {
		C int
	}
	type Outer struct {
		B Inner
	}
	config, err := NewToml(
		TomlFromByte[Outer]([]byte{}),
	)
	if err != nil {
		t.Fatalf("failed to create TomlConfig: %v", err)
	}
	err = config.SetPath([]string{"B", "C"}, 2)
	if err != nil {
		t.Fatalf("SetPath failed: %v", err)
	}
	expected := Outer{
		B: Inner{
			C: 2,
		},
	}
	if !reflect.DeepEqual(config.Data, expected) {
		t.Fatalf("SetPath result mismatch: got %v, want %v", config.Data, expected)
	}
}

func TestTomlSetPath(t *testing.T) {
	config := &TomlConfig[map[string]interface{}]{
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"c": 1,
				},
			},
		},
	}
	err := config.SetPath([]string{"a", "b", "name.with.comma"}, 2)
	if err != nil {
		t.Fatalf("SetPath failed: %v", err)
	}
	expected := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c":               1,
				"name.with.comma": 2,
			},
		},
	}
	if !reflect.DeepEqual(config.Data, expected) {
		t.Fatalf("SetPath result mismatch: got %v, want %v", config.Data, expected)
	}
}

func TestTomlSetPathExisting(t *testing.T) {
	config := &TomlConfig[map[string]interface{}]{
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"c":               1,
					"name.with.comma": 2,
				},
			},
		},
	}
	err := config.SetPath([]string{"a", "b", "name.with.comma"}, 3)
	if err != nil {
		t.Fatalf("SetPath failed: %v", err)
	}
	expected := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c":               1,
				"name.with.comma": 3,
			},
		},
	}
	if !reflect.DeepEqual(config.Data, expected) {
		t.Fatalf("SetPath result mismatch: got %v, want %v", config.Data, expected)
	}
}

func TestTomlSetPathWithList(t *testing.T) {
	config := &TomlConfig[map[string]interface{}]{
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b": []interface{}{
					1, 2, 3,
				},
			},
		},
	}
	err := config.SetPath([]string{"a", "b", "1"}, 20)
	if err == nil {
		t.Fatalf("SetPath should have failed when setting list index")
	}
}

func TestTomlDelete(t *testing.T) {
	config := &TomlConfig[map[string]interface{}]{
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"c": 1,
					"d": 2,
				},
			},
		},
	}
	err := config.DeletePath([]string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	expected := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"d": 2,
			},
		},
	}
	if !reflect.DeepEqual(config.Data, expected) {
		t.Fatalf("Delete result mismatch: got %v, want %v", config.Data, expected)
	}
}

func TestTomlDeleteNonExistent(t *testing.T) {
	config := &TomlConfig[map[string]interface{}]{
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"c": 1,
				},
			},
		},
	}
	err := config.DeletePath([]string{"a", "b", "d"})
	if err != nil {
		t.Fatalf("Delete should not fail for non-existent key: %v", err)
	}
	expected := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": 1,
			},
		},
	}
	if !reflect.DeepEqual(config.Data, expected) {
		t.Fatalf("Delete non-existent key altered data: got %v, want %v", config.Data, expected)
	}
}

func TestTomlDeletePathNotMap(t *testing.T) {
	config := &TomlConfig[map[string]interface{}]{
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b": 1,
			},
		},
	}
	err := config.DeletePath([]string{"a", "b", "c"})
	if err == nil {
		t.Fatalf("Delete should have failed when path is not a map")
	}
}

func TestTomlSetPathForStruct(t *testing.T) {
	type Inner struct {
		C int
	}
	type Outer struct {
		B Inner
	}
	config := &TomlConfig[Outer]{
		Data: Outer{
			B: Inner{
				C: 1,
			},
		},
	}
	err := config.SetPath([]string{"B", "C"}, 2)
	if err != nil {
		t.Fatalf("SetPath failed: %v", err)
	}
	expected := Outer{
		B: Inner{
			C: 2,
		},
	}
	if !reflect.DeepEqual(config.Data, expected) {
		t.Fatalf("SetPath result mismatch: got %v, want %v", config.Data, expected)
	}
}
