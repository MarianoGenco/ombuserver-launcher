package ombu

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Con OMBU_MPQ (un ombuserver.mpq) y OMBU_MPQ_GRAFICOS (la carpeta graficos-chat con los PNG que tiene adentro),
// compara lo que lee este lector con los originales. OMBU_MPQ_VOZ=<nombre>=<archivo> compara además una voz.
func TestLeerMPQ(t *testing.T) {
	ruta, dir := os.Getenv("OMBU_MPQ"), os.Getenv("OMBU_MPQ_GRAFICOS")
	if ruta == "" || dir == "" {
		t.Skip("sin OMBU_MPQ / OMBU_MPQ_GRAFICOS")
	}
	m, err := AbrirMPQ(ruta)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Cerrar()
	nombres, _ := filepath.Glob(filepath.Join(dir, "*.png"))
	for _, n := range nombres {
		want, _ := os.ReadFile(n)
		got, err := m.Leer(`data\ombu\launcher\chat\` + filepath.Base(n))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s: %v (leídos %d, esperados %d)", filepath.Base(n), err, len(got), len(want))
		}
	}
	if _, err := m.Leer(`data\ombu\launcher\chat\no-existe.png`); !os.IsNotExist(err) {
		t.Errorf("no-existe: %v", err)
	}
	if v := os.Getenv("OMBU_MPQ_VOZ"); v != "" {
		nombre, archivo, _ := bytes.Cut([]byte(v), []byte("="))
		want, _ := os.ReadFile(string(archivo))
		got, err := m.Leer(`data\ombu\voces\` + string(nombre))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("voz %s: %v (leídos %d, esperados %d)", nombre, err, len(got), len(want))
		}
	}
	t.Logf("%d figuras comparadas", len(nombres))
}
