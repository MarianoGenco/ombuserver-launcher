//go:build !windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Fuera de Windows (solo para probar): un config.json en la carpeta de configuración del usuario.
func rutaConfigPrueba() string {
	d, _ := os.UserConfigDir()
	return filepath.Join(d, "ombuserver-launcher", "config.json")
}

func leerConfigGuardada(c *Config) bool {
	b, err := os.ReadFile(rutaConfigPrueba())
	return err == nil && json.Unmarshal(b, c) == nil
}

func escribirConfigGuardada(c Config) error {
	os.MkdirAll(filepath.Dir(rutaConfigPrueba()), 0755)
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(rutaConfigPrueba(), b, 0644)
}
