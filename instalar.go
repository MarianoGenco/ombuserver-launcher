package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ombuserver-launcher/ombu"
)

// Instalación del juego completo (botón INSTALAR LOD): si la carpeta elegida no tiene Diablo II, se crea
// "<carpeta>\Diablo II" y se baja ahí el juego que publica el VPS web (Launcher/launcher/publicar-juego.sh).
// Cada archivo se verifica con el CRC32 del manifiesto; los que ya están completos no se vuelven a bajar, así
// que si se corta, al reintentar sigue desde donde quedó.

// URLDescargas: acá va la dirección de tu servidor web (se pone al compilar con -ldflags -X; ver ombu/ombu.go).
var URLDescargas = "http://IP-DEL-SERVIDOR-WEB/descargas/"

const (
	CarpetaDescarga    = "juego-1.14b/"
	NombreCarpetaJuego = "Diablo II"
)

// carpetaInstalacion: si la carpeta elegida ya es un Diablo II (o se llama "Diablo II") se instala o repara ahí
// mismo; si no, en una subcarpeta "Diablo II".
func carpetaInstalacion(base string, m *ombu.Manifiesto) string {
	if strings.EqualFold(filepath.Base(base), NombreCarpetaJuego) || pareceJuego(base, m) {
		return base
	}
	return filepath.Join(base, NombreCarpetaJuego)
}

// archivosJuego son los del juego base; si no se pudo bajar el manifiesto, la lista fija de publicar-juego.sh.
var archivosJuego = []string{"Game.exe", "D2.LNG", "Patch_D2.mpq", "SmackW32.dll", "binkw32.dll", "d2data.mpq",
	"d2exp.mpq", "d2char.mpq", "d2sfx.mpq", "d2speech.mpq", "d2music.mpq", "d2xtalk.mpq", "D2XMUSIC.MPQ", "D2XVIDEO.MPQ"}

// pareceJuego: la carpeta ya tiene algún archivo del juego (entonces se repara ahí, no se crea otra adentro).
func pareceJuego(dir string, m *ombu.Manifiesto) bool {
	for _, n := range nombresJuego(m) {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			return true
		}
	}
	return false
}

func nombresJuego(m *ombu.Manifiesto) []string {
	if m == nil {
		return archivosJuego
	}
	var r []string
	for _, f := range m.Files {
		if !f.Deprecated {
			r = append(r, f.Name)
		}
	}
	return r
}

// faltantesJuego: archivos del juego que no están o no tienen el tamaño publicado (sin calcular CRC: es rápido).
func faltantesJuego(dir string, m *ombu.Manifiesto) []string {
	var r []string
	if m == nil {
		for _, n := range archivosJuego {
			if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
				r = append(r, n)
			}
		}
		return r
	}
	for _, f := range m.Files {
		if f.Deprecated {
			continue
		}
		st, err := os.Stat(filepath.Join(dir, f.Name))
		if err != nil || st.Size() != f.ContentLength {
			r = append(r, f.Name)
		}
	}
	return r
}

// manifiestoJuego baja (una vez por sesión) la lista de archivos del juego publicada.
func (a *App) manifiestoJuego() (*ombu.Manifiesto, error) {
	a.mu.Lock()
	m := a.juego
	a.mu.Unlock()
	if m != nil {
		return m, nil
	}
	desc := &ombu.Cliente{Base: URLDescargas, HTTP: a.cliente.HTTP}
	t, err := desc.Texto(CarpetaDescarga + "manifest.json")
	if err != nil {
		return nil, err
	}
	var n ombu.Manifiesto
	if err := json.Unmarshal([]byte(t), &n); err != nil || len(n.Files) == 0 {
		return nil, errors.New("la lista de archivos del juego publicada no es válida")
	}
	a.mu.Lock()
	a.juego = &n
	a.mu.Unlock()
	return &n, nil
}

// Instalar baja el juego completo y deja configurada la carpeta nueva.
func (a *App) Instalar() error {
	cfg, liberar, err := a.ocupar()
	if err != nil {
		return err
	}
	defer liberar()

	base := cfg.Carpeta
	if base == "" {
		base, err = runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
			Title: "Elegí dónde instalar Diablo II (se crea la carpeta \"Diablo II\" adentro)",
		})
		if err != nil || base == "" {
			return err // cancelado
		}
	}
	a.avisar("Preparando la instalación…", -1)
	mp, err := a.manifiestoJuego()
	if err != nil {
		return sinConexion(err)
	}
	m := *mp
	desc := &ombu.Cliente{Base: URLDescargas, HTTP: a.cliente.HTTP}
	dir := carpetaInstalacion(base, mp)
	var total int64
	for _, f := range m.Files {
		if f.Name == "" || strings.ContainsAny(f.Name, `/\:`) || strings.HasPrefix(f.Name, ".") {
			return fmt.Errorf("la lista de archivos del juego trae un nombre no válido: %q", f.Name)
		}
		if !f.Deprecated {
			total += f.ContentLength
		}
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("no se pudo crear %s (¿la carpeta necesita permisos de administrador?)", dir)
	}
	var falta int64 // solo lo que hay que bajar (al reparar puede ser un archivo solo)
	for _, n := range faltantesJuego(dir, mp) {
		for _, f := range m.Files {
			if f.Name == n {
				falta += f.ContentLength
			}
		}
	}
	if libre, ok := espacioLibre(dir); ok && libre < uint64(falta)+200<<20 {
		return fmt.Errorf("no hay espacio suficiente: hacen falta %d MB y quedan %d MB libres", falta>>20, libre>>20)
	}

	barra := a.barra("Instalando Diablo II…")
	var hechos int64
	for _, f := range m.Files {
		if f.Deprecated {
			continue
		}
		destino := filepath.Join(dir, f.Name)
		if st, err := os.Stat(destino); err == nil && st.Size() == f.ContentLength {
			if crc, err := ombu.CRC32Archivo(destino); err == nil && strings.EqualFold(crc, f.CRC) {
				hechos += f.ContentLength
				barra(hechos, total)
				continue
			}
		}
		tmp := destino + ".tmp"
		inicio := hechos
		_, _, crc, err := desc.Bajar(CarpetaDescarga+url.PathEscape(f.Name), tmp, func(h, _ int64) { barra(inicio+h, total) })
		if err != nil {
			os.Remove(tmp)
			return sinConexion(err)
		}
		if !f.IgnoreCRC && !strings.EqualFold(crc, f.CRC) {
			os.Remove(tmp)
			return fmt.Errorf("%s se bajó con errores: volvé a tocar INSTALAR LOD (sigue desde donde quedó)", f.Name)
		}
		os.Remove(destino)
		if err := os.Rename(tmp, destino); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("no se pudo guardar %s: %v", f.Name, err)
		}
		hechos += f.ContentLength
	}

	a.mu.Lock()
	a.cfg.Carpeta = dir
	cfg = a.cfg
	err = guardarConfig(a.cfg)
	a.mu.Unlock()
	if err != nil {
		return err
	}
	return a.actualizarJuego(cfg)
}
