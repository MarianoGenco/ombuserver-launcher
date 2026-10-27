package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Con OMBU_JUEGO (carpeta de un Diablo II con d2data.mpq) arma la mano y, con OMBU_CURSOR_SALIDA, deja los archivos ahí.
func TestArmarCursor(t *testing.T) {
	carpeta := os.Getenv("OMBU_JUEGO")
	if carpeta == "" {
		t.Skip("sin OMBU_JUEGO")
	}
	a, err := armarCursor(carpeta)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", a["mano.json"])
	if dir := os.Getenv("OMBU_CURSOR_SALIDA"); dir != "" {
		for n, b := range a {
			os.WriteFile(filepath.Join(dir, n), b, 0o644)
		}
	}
}

// El manejador: sin carpeta 404; con el juego, mano.json y los PNG.
func TestServirCursor(t *testing.T) {
	carpeta := os.Getenv("OMBU_JUEGO")
	if carpeta == "" {
		t.Skip("sin OMBU_JUEGO")
	}
	app := &App{}
	c := nuevoCursor(app)
	pedir := func(ruta string) int {
		w := httptest.NewRecorder()
		c.ServeHTTP(w, httptest.NewRequest("GET", ruta, nil))
		return w.Code
	}
	if n := pedir("/img/cursor/mano.json"); n != 404 {
		t.Errorf("sin carpeta: %d", n)
	}
	app.cfg.Carpeta = carpeta
	for _, r := range []string{"/img/cursor/mano.json", "/img/cursor/mano-0.png", "/img/cursor/mano-7@2x.png"} {
		if n := pedir(r); n != 200 {
			t.Errorf("%s: %d", r, n)
		}
	}
	for _, r := range []string{"/img/cursor/mano-8.png", "/img/cursor/../x.png", "/img/cursor/otra.png"} {
		if n := pedir(r); n != 404 {
			t.Errorf("%s: %d", r, n)
		}
	}
}
