package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/pelletier/go-toml/v2"
)

type TomlConfig[T any] struct {
	Data T
}

type Option[T any] func(*T) error

// initNilMap initializes a nil map via reflection if the value is a map and nil.
func initNilMap(dataPtr interface{}) {
	v := reflect.ValueOf(dataPtr).Elem()
	if v.Kind() == reflect.Map && v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}
}

func NewToml[T any](opts ...Option[TomlConfig[T]]) (*TomlConfig[T], error) {
	cfg := &TomlConfig[T]{}

	// If T is a map, ensure it is initialized to avoid assignment to nil map
	initNilMap(&cfg.Data)

	for _, opt := range opts {
		err := opt(cfg)
		if err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func TomlFromConfigPath[T any](path string) Option[TomlConfig[T]] {
	return func(c *TomlConfig[T]) error {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("configuration file does not exist: %s", path)
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		err = toml.NewDecoder(f).Decode(&c.Data)
		if err != nil {
			return err
		}

		return nil
	}
}

func TomlFromByte[T any](data []byte) Option[TomlConfig[T]] {
	return func(c *TomlConfig[T]) error {
		err := toml.Unmarshal(data, &c.Data)
		if err != nil {
			return err
		}

		return nil
	}
}

func TomlFromStruct[T any](data T) Option[TomlConfig[T]] {
	return func(c *TomlConfig[T]) error {
		c.Data = data
		return nil
	}
}

func (c *TomlConfig[T]) Serialize() ([]byte, error) {
	data, err := toml.Marshal(c.Data)
	if err != nil {
		return nil, fmt.Errorf("unable to serialize configuration: %v", err)
	}

	return data, nil
}

func (c *TomlConfig[T]) SetPath(keys []string, value interface{}) error {
	v := reflect.ValueOf(&c.Data).Elem() // Use addressable value
	if v.Kind() != reflect.Map && v.Kind() != reflect.Struct {
		return fmt.Errorf("configuration is not a map or struct: %s", v.Kind())
	}

	// Traverse to the parent of the target key
	for i, key := range keys[:len(keys)-1] {
		switch v.Kind() {
		case reflect.Map:
			mapKey := reflect.ValueOf(key)
			elem := v.MapIndex(mapKey)
			if !elem.IsValid() {
				// Create a new map if path does not exist
				newMap := make(map[string]interface{})
				v.SetMapIndex(mapKey, reflect.ValueOf(newMap))
				elem = v.MapIndex(mapKey)
			}
			if elem.Kind() == reflect.Interface {
				elem = elem.Elem()
			}
			if elem.Kind() != reflect.Map && elem.Kind() != reflect.Struct {
				return fmt.Errorf("path %s is not a map or struct", key)
			}
			v = elem
		case reflect.Struct:
			field := v.FieldByName(key)
			if !field.IsValid() {
				return fmt.Errorf("field %s does not exist in struct at %v", key, keys[:i+1])
			}
			if field.Kind() == reflect.Ptr {
				if field.IsNil() {
					field.Set(reflect.New(field.Type().Elem()))
				}
				field = field.Elem()
			}
			if field.Kind() != reflect.Map && field.Kind() != reflect.Struct {
				return fmt.Errorf("field %s is not a map or struct", key)
			}
			v = field
		default:
			return fmt.Errorf("unsupported kind %s at %v", v.Kind(), keys[:i+1])
		}
	}

	lastKey := keys[len(keys)-1]
	switch v.Kind() {
	case reflect.Map:
		v.SetMapIndex(reflect.ValueOf(lastKey), reflect.ValueOf(value))
	case reflect.Struct:
		field := v.FieldByName(lastKey)
		if !field.IsValid() {
			return fmt.Errorf("field %s does not exist in struct", lastKey)
		}
		if !field.CanSet() {
			return fmt.Errorf("cannot set field %s in struct", lastKey)
		}
		val := reflect.ValueOf(value)
		if val.Type().AssignableTo(field.Type()) {
			field.Set(val)
		} else if val.Type().ConvertibleTo(field.Type()) {
			field.Set(val.Convert(field.Type()))
		} else {
			return fmt.Errorf("cannot assign value of type %s to field %s of type %s", val.Type(), lastKey, field.Type())
		}
	default:
		return fmt.Errorf("cannot set value at kind %s", v.Kind())
	}

	return nil
}

func (c *TomlConfig[T]) DeletePath(keys []string) error {
	v := reflect.ValueOf(&c.Data).Elem() // Use addressable value
	if v.Kind() != reflect.Map {
		return fmt.Errorf("configuration is not a map: %s", v.Kind())
	}

	// Traverse to the parent of the target key
	for _, key := range keys[:len(keys)-1] {
		v = v.MapIndex(reflect.ValueOf(key))
		if !v.IsValid() {
			// Path does not exist, nothing to delete
			return nil
		}
		if v.Kind() == reflect.Interface {
			v = v.Elem()
		}
		if v.Kind() != reflect.Map {
			return fmt.Errorf("path %s is not a map", key)
		}
	}

	parent := v
	lastKey := reflect.ValueOf(keys[len(keys)-1])
	if parent.Kind() == reflect.Interface {
		parent = parent.Elem()
	}
	if parent.Kind() == reflect.Map {
		parent.SetMapIndex(lastKey, reflect.Value{})
	}

	return nil
}

func (c *TomlConfig[T]) Save(path string) error {
	output, err := c.Serialize()
	if err != nil {
		return fmt.Errorf("unable to serialize configuration: %v", err)
	}

	// Create parent directories if they do not exist
	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return fmt.Errorf("unable to create configuration directory: %v", err)
	}

	// Save to file
	err = os.WriteFile(path, output, 0644)
	if err != nil {
		return fmt.Errorf("unable to write configuration file: %v", err)
	}

	return nil
}
