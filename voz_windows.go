//go:build windows

package main

import "golang.org/x/sys/windows"

var getAsyncKeyState = windows.NewLazySystemDLL("user32.dll").NewProc("GetAsyncKeyState")

// teclaApretada: estado actual de la tecla (o botón del mouse) en todo el sistema, tenga el foco quien lo tenga.
func teclaApretada(vk int) bool {
	r, _, _ := getAsyncKeyState.Call(uintptr(vk))
	return r&0x8000 != 0
}
