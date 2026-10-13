//go:build windows

package main

import "golang.org/x/sys/windows"

// espacioLibre devuelve los bytes libres en el disco de la carpeta (ok=false si no se pudo saber).
func espacioLibre(dir string) (uint64, bool) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, false
	}
	var libre, total, totalLibre uint64
	if err := windows.GetDiskFreeSpaceEx(p, &libre, &total, &totalLibre); err != nil {
		return 0, false
	}
	return libre, true
}
