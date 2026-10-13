//go:build windows

package main

import (
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"ombuserver-launcher/ombu"
)

// Autologueo del modo de prueba de carga (prueba_carga.go). Lo que se aprendió probando:
//   - el juego toma las letras como carácter (WM_CHAR); si además se manda la tecla (WM_KEYDOWN) las escribe dos veces;
//   - el Tab mandado desde afuera no cambia de campo (ni como tecla ni como carácter);
//   - al armar la pantalla de login, el juego lee la cuenta recordada ("Last BNet" en HKCU\...\Diablo II; Game.exe
//     0x430920): si hay una, la escribe y pone el cursor en la contraseña.
// Por eso, por cada ventana: se graba su cuenta como recordada, se abre el juego, se espera la pantalla de login y se
// escribe solo la contraseña. Recién entonces se pasa a la siguiente (si no, se pisarían la cuenta recordada).

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	enumWindows            = user32.NewProc("EnumWindows")
	getWindowThreadProcess = user32.NewProc("GetWindowThreadProcessId")
	isWindowVisible        = user32.NewProc("IsWindowVisible")
	postMessage            = user32.NewProc("PostMessageW")
	mapVirtualKey          = user32.NewProc("MapVirtualKeyW")
)

const (
	wmActivate     = 0x0006
	wmSetFocus     = 0x0007
	wmActivateApp  = 0x001C
	wmKeyDown      = 0x0100
	wmKeyUp        = 0x0101
	wmChar         = 0x0102
	vkBack         = 0x08
	vkReturn       = 0x0D
	cuentaGuardada = "Last BNet"
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

// escribir: cada letra como carácter (WM_CHAR).
func escribir(h uintptr, texto string) {
	for _, r := range texto {
		postMessage.Call(h, wmChar, uintptr(r), 1)
		time.Sleep(40 * time.Millisecond)
	}
}

// activar: los mensajes que manda Windows al hacer clic en la ventana.
func activar(h uintptr) {
	postMessage.Call(h, wmActivateApp, 1, 0)
	postMessage.Call(h, wmActivate, 1, 0)
	postMessage.Call(h, wmSetFocus, 0, 0)
	time.Sleep(200 * time.Millisecond)
}

// cuentaRecordada / recordarCuenta: la cuenta que el juego pone sola en la pantalla de login.
func cuentaRecordada() (string, bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, ombu.ClaveDiablo, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer k.Close()
	v, _, err := k.GetStringValue(cuentaGuardada)
	return v, err == nil
}

func recordarCuenta(cuenta string, hay bool) {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, ombu.ClaveDiablo, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	if hay {
		k.SetStringValue(cuentaGuardada, cuenta)
	} else {
		k.DeleteValue(cuentaGuardada)
	}
}

// loguearVentana: con la cuenta ya recordada, espera la pantalla de login del proceso y escribe la contraseña.
// Devuelve la ventana (0 si no apareció) para entrar después con el personaje.
func loguearVentana(pid int, c cuentaPrueba, p *pruebaCarga) uintptr {
	h := ventanaDe(pid)
	if h == 0 {
		anotarPrueba("%s: no apareció la ventana del juego (proceso %d) en 90 s", c.cuenta, pid)
		return 0
	}
	anotarPrueba("%s: ventana %#x del proceso %d; escribe la contraseña en %v", c.cuenta, h, pid, p.esperaLogin)
	time.Sleep(p.esperaLogin)
	if v, _, _ := isWindowVisible.Call(h); v == 0 {
		anotarPrueba("%s: la ventana ya no está (¿se cerró el juego?)", c.cuenta)
		return 0
	}
	activar(h)
	for i := 0; i < 16; i++ {
		tecla(h, vkBack)
	}
	escribir(h, c.clave)
	tecla(h, vkReturn)
	anotarPrueba("%s: contraseña escrita y Enter", c.cuenta)
	return h
}

// entrarPersonaje: Enter en la elección de personaje (entra con el primero).
func entrarPersonaje(h uintptr, c cuentaPrueba, espera time.Duration) {
	time.Sleep(espera)
	activar(h)
	tecla(h, vkReturn)
	anotarPrueba("%s: Enter en la elección de personaje", c.cuenta)
}
