//go:build windows

package main

import (
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Autologueo del modo de prueba de carga (prueba_carga.go): escribe en la ventana del juego como si fuera el teclado,
// con mensajes de ventana (no hace falta que tenga el foco).

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	enumWindows            = user32.NewProc("EnumWindows")
	getWindowThreadProcess = user32.NewProc("GetWindowThreadProcessId")
	isWindowVisible        = user32.NewProc("IsWindowVisible")
	postMessage            = user32.NewProc("PostMessageW")
	mapVirtualKey          = user32.NewProc("MapVirtualKeyW")
)

const (
	wmKeyDown = 0x0100
	wmKeyUp   = 0x0101
	wmChar    = 0x0102
	vkBack    = 0x08
	vkTab     = 0x09
	vkReturn  = 0x0D
)

// ventanaDe busca la ventana visible del proceso (espera hasta 90 s a que aparezca).
func ventanaDe(pid int) uintptr {
	var hallada uintptr
	cb := syscall.NewCallback(func(h, _ uintptr) uintptr {
		var p uint32
		getWindowThreadProcess.Call(h, uintptr(unsafe.Pointer(&p)))
		if int(p) == pid {
			if v, _, _ := isWindowVisible.Call(h); v != 0 {
				hallada = h
				return 0
			}
		}
		return 1
	})
	for fin := time.Now().Add(90 * time.Second); time.Now().Before(fin); time.Sleep(500 * time.Millisecond) {
		hallada = 0
		enumWindows.Call(cb, 0)
		if hallada != 0 {
			return hallada
		}
	}
	return 0
}

func tecla(h uintptr, vk uintptr) {
	sc, _, _ := mapVirtualKey.Call(vk, 0)
	postMessage.Call(h, wmKeyDown, vk, 1|sc<<16)
	time.Sleep(40 * time.Millisecond)
	postMessage.Call(h, wmKeyUp, vk, 1|sc<<16|1<<30|1<<31)
	time.Sleep(40 * time.Millisecond)
}

func escribir(h uintptr, texto string) {
	for _, r := range texto {
		postMessage.Call(h, wmChar, uintptr(r), 1)
		time.Sleep(40 * time.Millisecond)
	}
}

// autologuear: en la pantalla de Battle.net borra el campo de cuenta, escribe cuenta, Tab, contraseña, Enter; y
// después Enter para entrar con el primer personaje.
func autologuear(pid int, c cuentaPrueba, p *pruebaCarga) {
	h := ventanaDe(pid)
	if h == 0 {
		return
	}
	time.Sleep(p.esperaLogin)
	for i := 0; i < 16; i++ {
		tecla(h, vkBack)
	}
	escribir(h, c.cuenta)
	tecla(h, vkTab)
	for i := 0; i < 16; i++ {
		tecla(h, vkBack)
	}
	escribir(h, c.clave)
	tecla(h, vkReturn)
	time.Sleep(p.esperaPersonaje)
	tecla(h, vkReturn)
}
