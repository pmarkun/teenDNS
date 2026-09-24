package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func WriteAtomic[T any](path string, value T, mode os.FileMode) error {
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	contents = append(contents, '\n')

	temporary, err := os.CreateTemp(filepath.Dir(path), "gateway-*.json")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Chmod(temporaryPath, mode); err != nil {
		return fmt.Errorf("set config permissions: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
