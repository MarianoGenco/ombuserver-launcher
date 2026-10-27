//go:build windows

package main

import (
	"os"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Con OMBU_JUEGO (carpeta de un Diablo II): arma una ventana de clase wailsWindow como la de Wails, le pone la mano
// y revisa el cursor de la clase, el tamaño y punto activo a 96 y 144 DPI y la respuesta a WM_SETCURSOR.
func TestCursorVentana(t *testing.T) {
	carpeta := os.Getenv("OMBU_JUEGO")
	if carpeta == "" {
		t.Skip("sin OMBU_JUEGO")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defProc := user32.NewProc("DefWindowProcW")
	proc := syscall.NewCallback(func(h, m, w, l uintptr) uintptr { r, _, _ := defProc.Call(h, m, w, l); return r })
	type wndClassEx struct {
		size, style                   uint32
		wndProc                       uintptr
		clsExtra, wndExtra            int32
		instance, icon, cursor, fondo uintptr
		menuName, className           *uint16
		iconSm                        uintptr
	}
	clase, _ := windows.UTF16PtrFromString(claseWails)
	wc := wndClassEx{wndProc: proc, className: clase}
	wc.size = uint32(unsafe.Sizeof(wc))
	if r, _, err := user32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		t.Fatal("RegisterClassExW", err)
	}
	hwnd, _, err := user32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(clase)), 0, 0, 0, 0, 200, 200, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal("CreateWindowExW", err)
	}
	app := &App{}
	app.cfg.Carpeta = carpeta
	if !intentarCursorVentana(nuevoCursor(app)) {
		t.Fatal("no se puso la mano")
	}
	h := estadoCursor.hcursor.Load()
	gcl, _, _ := user32.NewProc("GetClassLongPtrW").Call(hwnd, uintptr(^uintptr(11))) // GCLP_HCURSOR = -12
	if gcl != h {
		t.Errorf("cursor de la clase %x, esperado %x", gcl, h)
	}
	r, _, _ := user32.NewProc("SendMessageW").Call(hwnd, wmSetCursor, hwnd, htClient|0x200<<16)
	if r != 1 {
		t.Errorf("WM_SETCURSOR (cliente) devolvió %d", r)
	}
	r, _, _ = user32.NewProc("SendMessageW").Call(hwnd, wmSetCursor, hwnd, htBorder|0x200<<16)
	if r != 1 {
		t.Errorf("WM_SETCURSOR (borde) devolvió %d", r)
	}
	info := func(hc uintptr) (int, int, uint32, uint32) {
		var ii iconInfo
		user32.NewProc("GetIconInfo").Call(hc, uintptr(unsafe.Pointer(&ii)))
		type bitmap struct {
			tipo, ancho, alto, anchoBytes int32
			planos, bits                  uint16
			datos                         uintptr
		}
		var bm bitmap
		gdi32.NewProc("GetObjectW").Call(ii.hbmColor, unsafe.Sizeof(bm), uintptr(unsafe.Pointer(&bm)))
		return int(bm.ancho), int(bm.alto), ii.xHotspot, ii.yHotspot
	}
	w, hh, x, y := info(h)
	t.Logf("96 DPI: %dx%d, punto activo (%d,%d)", w, hh, x, y)
	if w != 32 || hh != 29 || x != 1 || y != 4 {
		t.Errorf("96 DPI: %dx%d (%d,%d)", w, hh, x, y)
	}
	// dibujada sobre blanco: la esquina de abajo a la derecha (transparente) sigue blanca y la punta del dedo no
	{
		cab := bitmapV5Header{width: 32, height: -29, planes: 1, bitCount: 32}
		cab.size = uint32(unsafe.Sizeof(cab))
		var bits unsafe.Pointer
		dc, _, _ := gdi32.NewProc("CreateCompatibleDC").Call(0)
		bmp, _, _ := pCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&cab)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
		gdi32.NewProc("SelectObject").Call(dc, bmp)
		px := unsafe.Slice((*uint32)(bits), 32*29)
		for i := range px {
			px[i] = 0xFFFFFF
		}
		user32.NewProc("DrawIconEx").Call(dc, 0, 0, h, 32, 29, 0, 0, 3) // DI_NORMAL
		gdi32.NewProc("GdiFlush").Call()
		esquina, dedo := px[28*32+31]&0xFFFFFF, px[4*32+1]&0xFFFFFF
		t.Logf("esquina %06x, dedo %06x", esquina, dedo)
		if esquina != 0xFFFFFF || dedo == 0xFFFFFF {
			t.Errorf("transparencia: esquina %06x, dedo %06x", esquina, dedo)
		}
	}
	actualizarCursorVentana(192)
	w, hh, x, y = info(estadoCursor.hcursor.Load())
	t.Logf("192 DPI: %dx%d, punto activo (%d,%d)", w, hh, x, y)
	if w != 64 || hh != 58 || x != 2 || y != 8 {
		t.Errorf("192 DPI: %dx%d (%d,%d)", w, hh, x, y)
	}
	user32.NewProc("DestroyWindow").Call(hwnd)
}
