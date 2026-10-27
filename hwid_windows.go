//go:build windows

package main

import "golang.org/x/sys/windows/registry"

// idMaquina: el MachineGuid de Windows (estable por instalacion, no cambia con la IP ni la red).
func idMaquina() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, _ := k.GetStringValue("MachineGuid")
	return v
}
