//go:build windows

package main

import (
	"image"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// La página pone la mano con CSS (cursor.js), pero la ventana sin marco de Wails tiene una franja de un píxel abajo
// (Wails rellena el contenido con 1 px por un parpadeo de Windows) y a veces a la derecha que no cubre el WebView2:
// ahí Windows muestra el puntero de la clase de la ventana, la flecha. Acá se arma un HCURSOR con el mismo cuadro 0
// de la mano (mismo punto activo, escalado a los DPI de la ventana) y se le pone a la ventana del launcher: como
// cursor de su clase (lo que usa DefWindowProc en el área de cliente) y atendiendo WM_SETCURSOR para cualquier otra
// zona de la ventana. Las ventanas del WebView (clases Chrome_*) ponen su puntero ellas mismas y no se tocan. Si la
// mano no está (sin carpeta del juego todavía) no cambia nada y se reintenta cada tanto.

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	gdi32                 = windows.NewLazySystemDLL("gdi32.dll")
	pSetClassLongPtrW     = user32.NewProc("SetClassLongPtrW")
	pSetWindowLongPtrW    = user32.NewProc("SetWindowLongPtrW")
	pGetWindowLongPtrW    = user32.NewProc("GetWindowLongPtrW")
	pCallWindowProcW      = user32.NewProc("CallWindowProcW")
	pGetDpiForWindow      = user32.NewProc("GetDpiForWindow")
	pCreateIconIndirect   = user32.NewProc("CreateIconIndirect")
	pSetCursor            = user32.NewProc("SetCursor")
	pCreateDIBSection     = gdi32.NewProc("CreateDIBSection")
	pCreateBitmap         = gdi32.NewProc("CreateBitmap")
	pDeleteObject         = gdi32.NewProc("DeleteObject")
	llamadaBuscarVentanas = syscall.NewCallback(buscarVentana)
	llamadaBuscarHijas    = syscall.NewCallback(buscarHija)
	llamadaProcVentana    = syscall.NewCallback(procVentana)
)

const (
	gclpHCursor   = -12 // GCLP_HCURSOR
	gwlpWndProc   = -4  // GWLP_WNDPROC
	wmSetCursor   = 0x0020
	wmDPIChanged  = 0x02E0
	htClient      = 1
	htCaption     = 2
	htLeft        = 10 // HTLEFT..HTBOTTOMRIGHT: 10 a 17
	htBottomRight = 17
	htBorder      = 18
	claseWails    = "wailsWindow" // la ventana principal de Wails v2 (options.Windows.WindowClassName vacío)
)

var estadoCursor struct {
	mu       sync.Mutex // para armar y poner la mano (no lo toma procVentana, que corre en el hilo de la ventana)
	img      image.Image
	x, y     int
	ventana  windows.HWND
	dpi      uint32
	subclase bool

	hcursor   atomic.Uintptr // la mano para WM_SETCURSOR
	procViejo atomic.Uintptr // el procedimiento de Wails, al que se pasa todo lo demás
}

type iconInfo struct {
	fIcon              int32
	xHotspot, yHotspot uint32
	hbmMask, hbmColor  uintptr
}

type bitmapV5Header struct {
	size                                    uint32
	width, height                           int32
	planes, bitCount                        uint16
	compression, sizeImage                  uint32
	xPelsPerMeter, yPelsPerMeter            int32
	clrUsed, clrImportant                   uint32
	redMask, greenMask, blueMask, alphaMask uint32
	csType                                  uint32
	endpoints                               [36]byte
	gammaRed, gammaGreen, gammaBlue         uint32
	intent, profileData, profileSize        uint32
	reserved                                uint32
}

// crearHCursor arma un cursor de Windows con transparencia (32 bits con alfa, como en el KB318876 de Microsoft),
// escalando la imagen píxel por píxel a escala (1 = 96 DPI).
func crearHCursor(img image.Image, hx, hy int, escala float64) uintptr {
	b := img.Bounds()
	ancho := max(1, int(math.Round(float64(b.Dx())*escala)))
	alto := max(1, int(math.Round(float64(b.Dy())*escala)))
	cab := bitmapV5Header{
		width: int32(ancho), height: -int32(alto), planes: 1, bitCount: 32, compression: 3, // BI_BITFIELDS, de arriba abajo
		redMask: 0x00FF0000, greenMask: 0x0000FF00, blueMask: 0x000000FF, alphaMask: 0xFF000000,
	}
	cab.size = uint32(unsafe.Sizeof(cab))
	var bits unsafe.Pointer
	color, _, _ := pCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&cab)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if color == 0 || bits == nil {
		return 0
	}
	defer pDeleteObject.Call(color)
	px := unsafe.Slice((*uint32)(bits), ancho*alto)
	for y := 0; y < alto; y++ {
		for x := 0; x < ancho; x++ {
			sx := b.Min.X + min(b.Dx()-1, int(float64(x)/escala))
			sy := b.Min.Y + min(b.Dy()-1, int(float64(y)/escala))
			r, g, bl, a := img.At(sx, sy).RGBA()
			a8 := a >> 8
			if a8 == 0 {
				px[y*ancho+x] = 0
				continue
			}
			// premultiplicado (la mano solo tiene alfa 0 o 255, así que da lo mismo, pero es lo que espera Windows)
			px[y*ancho+x] = a8<<24 | (r>>8)<<16 | (g>>8)<<8 | bl>>8
		}
	}
	mascara := make([]byte, ((ancho+15)/16)*2*alto) // monocroma en cero: manda el alfa
	mono, _, _ := pCreateBitmap.Call(uintptr(ancho), uintptr(alto), 1, 1, uintptr(unsafe.Pointer(&mascara[0])))
	if mono == 0 {
		return 0
	}
	defer pDeleteObject.Call(mono)
	ii := iconInfo{
		fIcon:    0,
		xHotspot: uint32(min(ancho-1, int(math.Round(float64(hx)*escala)))),
		yHotspot: uint32(min(alto-1, int(math.Round(float64(hy)*escala)))),
		hbmMask:  mono, hbmColor: color,
	}
	h, _, _ := pCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	return h
}

// buscarVentana (EnumWindows): la ventana de Wails de este proceso.
func buscarVentana(hwnd windows.HWND, param unsafe.Pointer) uintptr {
	var pid uint32
	windows.GetWindowThreadProcessId(hwnd, &pid)
	if pid != uint32(os.Getpid()) || claseDe(hwnd) != claseWails {
		return 1
	}
	*(*windows.HWND)(param) = hwnd
	return 0
}

// buscarHija (EnumChildWindows): a las hijas de este proceso que no son del WebView también se les pone la mano.
func buscarHija(hwnd windows.HWND, _ uintptr) uintptr {
	var pid uint32
	windows.GetWindowThreadProcessId(hwnd, &pid)
	if pid == uint32(os.Getpid()) && !strings.HasPrefix(claseDe(hwnd), "Chrome_") {
		setClassCursor(hwnd, estadoCursor.hcursor.Load())
	}
	return 1
}

func claseDe(hwnd windows.HWND) string {
	var b [128]uint16
	n, err := windows.GetClassName(hwnd, &b[0], int32(len(b)))
	if err != nil || n <= 0 {
		return ""
	}
	return windows.UTF16ToString(b[:n])
}

func setClassCursor(hwnd windows.HWND, hcursor uintptr) {
	indice := gclpHCursor
	pSetClassLongPtrW.Call(uintptr(hwnd), uintptr(indice), hcursor)
}

// procVentana: la subclase de la ventana del launcher. En WM_SETCURSOR sobre la ventana misma (no sobre el WebView)
// pone la mano en el área de cliente y en los bordes (la ventana no se redimensiona, así que no hay flechas de
// tamaño que perder); el resto sigue a la ventana de Wails.
func procVentana(hwnd, msg, wparam, lparam uintptr) uintptr {
	viejo, hcursor := estadoCursor.procViejo.Load(), estadoCursor.hcursor.Load()
	switch msg {
	case wmSetCursor:
		ht := lparam & 0xFFFF
		if hcursor != 0 && wparam == hwnd && (ht == htClient || ht == htCaption || ht == htBorder || (ht >= htLeft && ht <= htBottomRight)) {
			pSetCursor.Call(hcursor)
			return 1
		}
	case wmDPIChanged: // otra pantalla con otra escala: la mano del tamaño que corresponde
		go actualizarCursorVentana(uint32(wparam & 0xFFFF))
	}
	r, _, _ := pCallWindowProcW.Call(viejo, hwnd, msg, wparam, lparam)
	return r
}

// actualizarCursorVentana arma la mano para dpi (0 = el de la ventana) y se la pone a la ventana y a sus hijas.
func actualizarCursorVentana(dpi uint32) {
	estadoCursor.mu.Lock()
	defer estadoCursor.mu.Unlock()
	hwnd := estadoCursor.ventana
	if hwnd == 0 || estadoCursor.img == nil {
		return
	}
	if dpi == 0 {
		if pGetDpiForWindow.Find() == nil { // Windows 10 1607 en adelante
			d, _, _ := pGetDpiForWindow.Call(uintptr(hwnd))
			dpi = uint32(d)
		}
		if dpi == 0 {
			dpi = 96
		}
	}
	if dpi == estadoCursor.dpi && estadoCursor.hcursor.Load() != 0 {
		return
	}
	h := crearHCursor(estadoCursor.img, estadoCursor.x, estadoCursor.y, float64(dpi)/96)
	if h == 0 {
		return
	}
	// el anterior no se destruye: algún WM_SETCURSOR en curso podría estar usándolo (es un puñado de bytes)
	estadoCursor.hcursor.Store(h)
	estadoCursor.dpi = dpi
	setClassCursor(hwnd, h)
	windows.EnumChildWindows(hwnd, llamadaBuscarHijas, nil)
}

// intentarCursorVentana: true cuando quedó puesto (o no hay nada más que hacer).
func intentarCursorVentana(c *cursorMano) bool {
	img, x, y, err := c.manoQuieta()
	if err != nil {
		return false // todavía sin mano: queda la flecha
	}
	var hwnd windows.HWND
	windows.EnumWindows(llamadaBuscarVentanas, unsafe.Pointer(&hwnd))
	if hwnd == 0 {
		return false // la ventana todavía no está
	}
	estadoCursor.mu.Lock()
	estadoCursor.img, estadoCursor.x, estadoCursor.y, estadoCursor.ventana = img, x, y, hwnd
	estadoCursor.mu.Unlock()
	actualizarCursorVentana(0)
	estadoCursor.mu.Lock()
	defer estadoCursor.mu.Unlock()
	if estadoCursor.hcursor.Load() == 0 {
		return false
	}
	if !estadoCursor.subclase {
		// primero se guarda el procedimiento de Wails y recién después se pone el nuestro: el hilo de la ventana
		// puede llamarlo apenas queda puesto. Si no se puede, queda el cursor de la clase (cubre el área de cliente).
		indice := gwlpWndProc
		if viejo, _, _ := pGetWindowLongPtrW.Call(uintptr(hwnd), uintptr(indice)); viejo != 0 {
			estadoCursor.procViejo.Store(viejo)
			if r, _, _ := pSetWindowLongPtrW.Call(uintptr(hwnd), uintptr(indice), llamadaProcVentana); r != 0 {
				estadoCursor.subclase = true
			}
		}
	}
	return true
}

// ponerCursorVentana se llama al arrancar: espera la ventana y la mano (la mano puede tardar si todavía no se eligió
// la carpeta del juego) y se las pone.
func ponerCursorVentana(c *cursorMano) {
	for i := 0; i < 120; i++ {
		if intentarCursorVentana(c) {
			return
		}
		espera := 2 * time.Second
		if i >= 15 {
			espera = 15 * time.Second
		}
		time.Sleep(espera)
	}
}
