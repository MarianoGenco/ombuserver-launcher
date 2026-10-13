//go:build windows

package main

import "golang.org/x/sys/windows"

// esperarProceso espera (como mucho 15 s) a que termine el proceso pid.
func esperarProceso(pid int) {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return // ya terminó
	}
	defer windows.CloseHandle(h)
	windows.WaitForSingleObject(h, 15000)
}
