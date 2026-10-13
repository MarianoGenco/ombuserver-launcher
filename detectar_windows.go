//go:build windows

package main

import "golang.org/x/sys/windows/registry"

// carpetaInstalada es la carpeta que anotó el instalador de Diablo II (si lo hay).
func carpetaInstalada() string {
	for _, raiz := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		k, err := registry.OpenKey(raiz, `Software\Blizzard Entertainment\Diablo II`, registry.QUERY_VALUE|registry.WOW64_32KEY)
		if err != nil {
			continue
		}
		v, _, err := k.GetStringValue("InstallPath")
		k.Close()
		if err == nil && v != "" {
			return v
		}
	}
	return ""
}
