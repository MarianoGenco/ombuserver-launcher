//go:build !windows

package main

import "os"

// idMaquina: en Linux/dev, /etc/machine-id. Solo para compilar y probar fuera de Windows.
func idMaquina() string {
	b, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return ""
	}
	return string(b)
}
