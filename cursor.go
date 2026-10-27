package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"ombuserver-launcher/ombu"
)

// Puntero del mouse: la mano (el guantelete) de Diablo II. Como las figuras del chat, sale de los archivos del juego
// y no va en el repo ni en el .exe: se lee del d2data.mpq del jugador (data\global\ui\CURSOR\ohand.dc6, con la paleta
// del Acto I) y se convierte a PNG en el momento. ohand.dc6 trae 8 cuadros: el 0 es la mano que señala y del 1 al 7
// se va abriendo, que es lo que el juego anima mientras se aprieta el botón. Los cuadros tienen distinto tamaño y
// corrimiento, así que se pegan todos en un mismo lienzo, alineados como los dibuja el juego, para que la mano no
// salte al animarse. La ventana pide:
//
//	/img/cursor/mano.json        cuántos cuadros hay y dónde está la punta del dedo (el punto activo)
//	/img/cursor/mano-N.png       el cuadro N
//	/img/cursor/mano-N@2x.png    el cuadro N al doble, píxel por píxel (pantallas con escala)
//
// Sin juego todavía (carpeta sin elegir) responde 404 y la ventana deja el puntero de Windows.

const (
	ladoMaxCursor = 32
	prefijoCursor = "/img/cursor/"
	cursorEnMPQ   = `data\global\ui\CURSOR\ohand.dc6`
	paletaEnMPQ   = `data\global\palette\ACT1\pal.dat`
)

var nombreCursor = regexp.MustCompile(`^mano(-[0-9]{1,2}(@2x)?\.png|\.json)$`)

type cursorMano struct {
	app *App

	mu       sync.Mutex
	carpeta  string            // de qué juego salió lo que hay en archivos
	archivos map[string][]byte // mano.json y los PNG
	fallo    time.Time         // último intento fallido (para no abrir los MPQ a cada pedido)
}

func nuevoCursor(app *App) *cursorMano { return &cursorMano{app: app} }

// leerDelJuego busca el archivo en los MPQ del juego en el orden en que los carga Diablo II.
func leerDelJuego(carpeta, interno string) ([]byte, error) {
	for _, nombre := range []string{"Patch_D2.mpq", "d2exp.mpq", "d2data.mpq"} {
		m, err := ombu.AbrirMPQ(filepath.Join(carpeta, nombre))
		if err != nil {
			continue
		}
		b, err := m.Leer(interno)
		m.Cerrar()
		if err == nil {
			return b, nil
		}
	}
	return nil, errors.New(interno + ": no está en los MPQ del juego")
}

type cuadroDC6 struct {
	ancho, alto, x, y int
	px                []int // índices de la paleta; -1 = transparente
}

// leerDC6 decodifica todos los cuadros de un .dc6 (imagen 2D de Diablo II: filas de abajo hacia arriba, tiradas de
// transparentes con el bit alto y tiradas de píxeles con el largo adelante, 0x80 = fin de fila).
func leerDC6(d []byte) ([]cuadroDC6, error) {
	malo := errors.New("dc6 inválido")
	if len(d) < 24 {
		return nil, malo
	}
	le := binary.LittleEndian
	dirs, porDir := int(int32(le.Uint32(d[16:]))), int(int32(le.Uint32(d[20:])))
	n := dirs * porDir
	if n <= 0 || n > 256 || 24+4*n > len(d) {
		return nil, malo
	}
	var cuadros []cuadroDC6
	for i := 0; i < n; i++ {
		p := int(le.Uint32(d[24+4*i:]))
		if p < 0 || p+32 > len(d) {
			return nil, malo
		}
		cab := func(k int) int { return int(int32(le.Uint32(d[p+4*k:]))) }
		volteado, ancho, alto, ox, oy, largo := cab(0), cab(1), cab(2), cab(3), cab(4), cab(7)
		if ancho <= 0 || alto <= 0 || ancho > 256 || alto > 256 || largo < 0 || p+32+largo > len(d) {
			return nil, malo
		}
		c := cuadroDC6{ancho: ancho, alto: alto, x: ox, y: oy, px: make([]int, ancho*alto)}
		for k := range c.px {
			c.px[k] = -1
		}
		crudo := d[p+32 : p+32+largo]
		x, y, dy := 0, alto-1, -1
		if volteado != 0 {
			y, dy = 0, 1
		}
		for j := 0; j < len(crudo) && y >= 0 && y < alto; {
			b := crudo[j]
			j++
			switch {
			case b == 0x80:
				x, y = 0, y+dy
			case b&0x80 != 0:
				x += int(b & 0x7f)
			default:
				for k := 0; k < int(b) && j+k < len(crudo); k++ {
					if x < ancho {
						c.px[y*ancho+x] = int(crudo[j+k])
					}
					x++
				}
				j += int(b)
			}
		}
		cuadros = append(cuadros, c)
	}
	return cuadros, nil
}

func pngDe(img image.Image) []byte {
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

// armar convierte la mano del juego que está en carpeta.
func armarCursor(carpeta string) (map[string][]byte, error) {
	dc6, err := leerDelJuego(carpeta, cursorEnMPQ)
	if err != nil {
		return nil, err
	}
	pal, err := leerDelJuego(carpeta, paletaEnMPQ)
	if err != nil {
		return nil, err
	}
	if len(pal) < 768 {
		return nil, errors.New("paleta inválida")
	}
	cuadros, err := leerDC6(dc6)
	if err != nil {
		return nil, err
	}
	// Lienzo común: el juego dibuja cada cuadro con la esquina de abajo a la izquierda en (x, y) respecto del mouse.
	minX, minY, maxX, maxY := 1<<30, 1<<30, -1<<30, -1<<30
	for _, c := range cuadros {
		minX, minY = min(minX, c.x), min(minY, c.y-c.alto)
		maxX, maxY = max(maxX, c.x+c.ancho), max(maxY, c.y)
	}
	// A lo sumo 32x32: Chromium cambia por la flecha de Windows todo puntero de más de 32 px (en px de CSS) cuando
	// la imagen se saldría de la ventana, o sea en una franja junto a los bordes (abajo y a la derecha se veía la
	// flecha, 2026-10-14). ohand.dc6 ocupa 33 de ancho entre todos sus cuadros (los de la mano abierta empiezan un
	// píxel más a la izquierda): esos se corren un píxel adentro, lo que no se nota en la animación.
	minX, minY = max(minX, maxX-ladoMaxCursor), max(minY, maxY-ladoMaxCursor)
	ancho, alto := maxX-minX, maxY-minY
	ubicar := func(c cuadroDC6) (int, int) {
		return max(0, min(c.x-minX, ancho-c.ancho)), max(0, min(c.y-c.alto-minY, alto-c.alto))
	}
	colores := make(color.Palette, 256)
	for i := range colores { // pal.dat: 256 colores en BGR
		colores[i] = color.NRGBA{pal[i*3+2], pal[i*3+1], pal[i*3], 255}
	}
	archivos := map[string][]byte{}
	for n, c := range cuadros {
		img1 := image.NewNRGBA(image.Rect(0, 0, ancho, alto))
		img2 := image.NewNRGBA(image.Rect(0, 0, ancho*2, alto*2))
		x0, y0 := ubicar(c)
		for y := 0; y < c.alto; y++ {
			for x := 0; x < c.ancho; x++ {
				if i := c.px[y*c.ancho+x]; i >= 0 {
					img1.Set(x0+x, y0+y, colores[i])
					for k := 0; k < 4; k++ {
						img2.Set((x0+x)*2+k%2, (y0+y)*2+k/2, colores[i])
					}
				}
			}
		}
		archivos[fmt.Sprintf("mano-%d.png", n)] = pngDe(img1)
		archivos[fmt.Sprintf("mano-%d@2x.png", n)] = pngDe(img2)
	}
	// Punto activo: la punta del dedo del cuadro 0 (su primer píxel de arriba, el de más a la izquierda), un píxel
	// adentro para que caiga sobre el dedo y no en el borde.
	c := cuadros[0]
	x0, y0 := ubicar(c)
	px, py := 0, 0
buscar:
	for y := 0; y < c.alto; y++ {
		for x := 0; x < c.ancho; x++ {
			if c.px[y*c.ancho+x] >= 0 {
				px, py = x, y
				break buscar
			}
		}
	}
	info, _ := json.Marshal(map[string]int{
		"cuadros": len(cuadros), "ancho": ancho, "alto": alto,
		"x": min(x0+px+1, ancho-1), "y": min(y0+py+1, alto-1),
	})
	archivos["mano.json"] = info
	return archivos, nil
}

func (m *cursorMano) archivo(nombre string) ([]byte, error) {
	m.app.mu.Lock()
	carpeta := m.app.cfg.Carpeta
	m.app.mu.Unlock()
	if carpeta == "" {
		return nil, errors.New("sin carpeta del juego")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.archivos == nil || m.carpeta != carpeta {
		if time.Since(m.fallo) < 5*time.Second {
			return nil, errors.New("sin cursor")
		}
		a, err := armarCursor(carpeta)
		if err != nil {
			m.fallo = time.Now()
			return nil, err
		}
		m.archivos, m.carpeta = a, carpeta
	}
	b, ok := m.archivos[nombre]
	if !ok {
		return nil, errors.New("no existe")
	}
	return b, nil
}

// manoQuieta devuelve el cuadro 0 (la mano que señala, en el lienzo común) y su punto activo, para el puntero de
// Windows en los bordes de la ventana que no cubre la página (cursor_windows.go).
func (m *cursorMano) manoQuieta() (image.Image, int, int, error) {
	info, err := m.archivo("mano.json")
	if err != nil {
		return nil, 0, 0, err
	}
	var i struct{ X, Y int }
	if err := json.Unmarshal(info, &i); err != nil {
		return nil, 0, 0, err
	}
	b, err := m.archivo("mano-0.png")
	if err != nil {
		return nil, 0, 0, err
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, 0, 0, err
	}
	return img, i.X, i.Y, nil
}

func (m *cursorMano) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	nombre, ok := strings.CutPrefix(r.URL.Path, prefijoCursor)
	if !ok || !nombreCursor.MatchString(nombre) {
		http.NotFound(w, r)
		return
	}
	b, err := m.archivo(nombre)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	tipo := "image/png"
	if strings.HasSuffix(nombre, ".json") {
		tipo = "application/json"
	}
	w.Header().Set("Content-Type", tipo)
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, nombre, time.Time{}, bytes.NewReader(b))
}
