package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"ombuserver-launcher/ombu"
)

// Figuras del chat (img/chat/*.png): personajes, rangos, la cruz del ignorado, las flechas y el pasto del lobby. Salen
// de los archivos de Diablo II, así que no van en el repo ni dentro del .exe: las publica el servidor web en
// contenido/chat/ junto con manifiesto.json (nombre -> sha256), y el launcher las baja, verifica y guarda en
// %LOCALAPPDATA%\OmbuServer Launcher\chat. La ventana las pide como siempre: Wails llama a este manejador con todo lo
// que no está adentro del .exe.

const prefijoGraficos = "/img/chat/"

var nombreGrafico = regexp.MustCompile(`^[a-z0-9@-]{1,64}\.png$`)

type graficosChat struct {
	mu         sync.Mutex
	manifiesto map[string]string // nombre -> sha256 (se baja una vez por sesión)
	hasta      time.Time         // si el manifiesto no se pudo bajar, no reintentar antes de esto
	cli        *http.Client
}

func nuevosGraficos() *graficosChat {
	return &graficosChat{cli: &http.Client{Timeout: 30 * time.Second}}
}

func carpetaGraficos() string {
	d, err := os.UserCacheDir() // en Windows, %LOCALAPPDATA%
	if err != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "OmbuServer Launcher", "chat")
}

func (g *graficosChat) leerManifiesto() (map[string]string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.manifiesto != nil {
		return g.manifiesto, nil
	}
	if time.Now().Before(g.hasta) {
		return nil, errors.New("sin manifiesto de gráficos")
	}
	m, err := g.bajarManifiesto()
	if err != nil {
		g.hasta = time.Now().Add(30 * time.Second)
		return nil, err
	}
	g.manifiesto = m
	return m, nil
}

func (g *graficosChat) bajarManifiesto() (map[string]string, error) {
	b, err := g.bajar("manifiesto.json", 64<<10)
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (g *graficosChat) bajar(nombre string, tope int64) ([]byte, error) {
	r, err := g.cli.Get(ombu.URLContenido + "chat/" + nombre)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return nil, errors.New("estado " + r.Status)
	}
	return io.ReadAll(io.LimitReader(r.Body, tope))
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// grafico devuelve el PNG: el guardado si coincide con el manifiesto; si no, lo baja, lo verifica y lo guarda.
// Sin conexión, usa el guardado aunque no se pueda comparar.
func (g *graficosChat) grafico(nombre string) ([]byte, error) {
	ruta := filepath.Join(carpetaGraficos(), nombre)
	guardado, errLocal := os.ReadFile(ruta)
	m, err := g.leerManifiesto()
	if err != nil {
		if errLocal == nil {
			return guardado, nil
		}
		return nil, err
	}
	esperado, ok := m[nombre]
	if !ok {
		return nil, os.ErrNotExist
	}
	if errLocal == nil && sha256Hex(guardado) == esperado {
		return guardado, nil
	}
	b, err := g.bajar(nombre, 4<<20)
	if err != nil {
		if errLocal == nil {
			return guardado, nil
		}
		return nil, err
	}
	if sha256Hex(b) != esperado {
		return nil, errors.New(nombre + ": no coincide con el manifiesto")
	}
	if os.MkdirAll(filepath.Dir(ruta), 0755) == nil {
		tmp := ruta + ".tmp"
		if os.WriteFile(tmp, b, 0644) == nil {
			os.Rename(tmp, ruta)
		}
	}
	return b, nil
}

// precargar baja en segundo plano todas las figuras, para que el chat las tenga al abrirlo.
func (g *graficosChat) precargar() {
	m, err := g.leerManifiesto()
	if err != nil {
		return
	}
	for nombre := range m {
		if nombreGrafico.MatchString(nombre) {
			g.grafico(nombre)
		}
	}
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
