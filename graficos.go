package main

import (
	"bytes"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"ombuserver-launcher/ombu"
)

// Figuras del chat (img/chat/*.png): personajes, rangos, la cruz del ignorado, las flechas y el pasto del lobby. Salen
// de los archivos de Diablo II, así que no van en el repo ni dentro del .exe: viajan adentro de ombuserver.mpq
// (data\ombu\launcher\chat\, cifradas como el resto), el mismo que el launcher ya baja y mantiene al día. La ventana
// las pide como siempre: Wails llama a este manejador con todo lo que no está adentro del .exe.

const (
	prefijoGraficos = "/img/chat/"
	carpetaEnMPQ    = `data\ombu\launcher\chat\`
)

var nombreGrafico = regexp.MustCompile(`^[a-z0-9@-]{1,64}\.png$`)

type graficosChat struct {
	app *App

	mu        sync.Mutex
	cache     map[string][]byte // lo ya leído del MPQ en esta sesión
	sincroDel time.Time         // último intento de bajar el MPQ (para no reintentar a cada imagen)
}

func nuevosGraficos(app *App) *graficosChat {
	return &graficosChat{app: app, cache: map[string][]byte{}}
}

func (g *graficosChat) carpetaDatos() string {
	g.app.mu.Lock()
	carpeta := g.app.cfg.Carpeta
	g.app.mu.Unlock()
	return ombu.CarpetaDatos(carpeta, "")
}

// leerDelMPQ abre el MPQ, saca la figura y lo cierra enseguida (abierto trabaría su actualización).
func (g *graficosChat) leerDelMPQ(nombre string) ([]byte, error) {
	m, err := ombu.AbrirMPQ(filepath.Join(g.carpetaDatos(), ombu.ArchivoMPQ))
	if err != nil {
		return nil, err
	}
	defer m.Cerrar()
	return m.Leer(carpetaEnMPQ + nombre)
}

// sincronizar baja el MPQ publicado si el que hay no está o es otro (a lo sumo una vez por minuto).
func (g *graficosChat) sincronizar() {
	g.mu.Lock()
	if time.Since(g.sincroDel) < time.Minute {
		g.mu.Unlock()
		return
	}
	g.sincroDel = time.Now()
	g.mu.Unlock()
	if cambio, err := g.app.cliente.SincronizarMPQ(g.carpetaDatos(), nil); err == nil && cambio {
		g.mu.Lock()
		g.cache = map[string][]byte{}
		g.mu.Unlock()
	}
}

func (g *graficosChat) grafico(nombre string) ([]byte, error) {
	g.mu.Lock()
	b, ok := g.cache[nombre]
	g.mu.Unlock()
	if ok {
		return b, nil
	}
	b, err := g.leerDelMPQ(nombre)
	if err != nil { // sin MPQ, o uno anterior a las figuras: se baja el publicado y se reintenta
		g.sincronizar()
		if b, err = g.leerDelMPQ(nombre); err != nil {
			return nil, err
		}
	}
	g.mu.Lock()
	g.cache[nombre] = b
	g.mu.Unlock()
	return b, nil
}

// precargar: al abrir el launcher, si el MPQ no tiene las figuras, lo baja en segundo plano para que el chat las
// tenga al abrirlo.
func (g *graficosChat) precargar() {
	g.grafico("pasto.png")
}

func (g *graficosChat) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	nombre, ok := strings.CutPrefix(r.URL.Path, prefijoGraficos)
	if !ok || !nombreGrafico.MatchString(nombre) {
		http.NotFound(w, r)
		return
	}
	b, err := g.grafico(nombre)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	http.ServeContent(w, r, nombre, time.Time{}, bytes.NewReader(b))
}
