package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Actualización del propio launcher. El servidor publica launcher/version.json:
//
//	{"version": "0.3.1", "archivo": "OmbuServer Launcher.exe", "sha256": "...", "tamano": 11617792}
//
// (lo arma publicar.sh). Si la versión publicada es más nueva que Version, el botón JUGAR pasa a
// ACTUALIZAR: se baja el .exe nuevo, se verifica su sha256, el que está corriendo se renombra a
// .viejo (Windows deja renombrar un .exe abierto, no pisarlo), el nuevo toma su lugar y se abre.
// El nuevo borra el .viejo al arrancar.

const carpetaLauncher = "launcher/"

// VersionLauncher es lo que publica el servidor.
type VersionLauncher struct {
	Version string `json:"version"`
	Archivo string `json:"archivo"`
	SHA256  string `json:"sha256"`
	Tamano  int64  `json:"tamano"`
}

// versionPublicada devuelve la versión publicada del launcher (nil si no hay ninguna publicada).
func (a *App) versionPublicada() (*VersionLauncher, error) {
	t, err := a.cliente.Texto(carpetaLauncher + "version.json")
	if err != nil {
		if strings.Contains(err.Error(), "estado 404") {
			return nil, nil
		}
		return nil, err
	}
	var v VersionLauncher
	if err := json.Unmarshal([]byte(t), &v); err != nil {
		return nil, fmt.Errorf("version.json del launcher: %v", err)
	}
	return &v, nil
}

// masNueva dice si la versión a (por ejemplo "0.3.1") es posterior a b.
func masNueva(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// actualizarLauncher baja el launcher nuevo, se reemplaza y abre el nuevo. Si sale bien, el que
// llama tiene que cerrar este proceso.
func (a *App) actualizarLauncher(v *VersionLauncher) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.EvalSymlinks(exe)
	nuevo := exe + ".nuevo"
	_, _, _, err = a.cliente.Bajar(carpetaLauncher+v.Archivo, nuevo, a.barra("Actualizando el launcher…"))
	if err != nil {
		os.Remove(nuevo)
		return sinConexion(err)
	}
	if s, err := sha256Archivo(nuevo); err != nil || !strings.EqualFold(s, v.SHA256) {
		os.Remove(nuevo)
		return errors.New("el launcher bajado no coincide con el publicado: volvé a intentar")
	}
	viejo := exe + ".viejo"
	os.Remove(viejo)
	if err := os.Rename(exe, viejo); err != nil {
		os.Remove(nuevo)
		return fmt.Errorf("no se pudo reemplazar el launcher (¿está en una carpeta protegida?): %v", err)
	}
	if err := os.Rename(nuevo, exe); err != nil {
		os.Rename(viejo, exe)
		os.Remove(nuevo)
		return fmt.Errorf("no se pudo reemplazar el launcher: %v", err)
	}
	cmd := exec.Command(exe, fmt.Sprintf("--esperar=%d", os.Getpid()))
	cmd.Dir = filepath.Dir(exe)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("el launcher se actualizó, pero no se pudo abrir: abrilo de nuevo (%v)", err)
	}
	return nil
}

func sha256Archivo(ruta string) (string, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// recienActualizado: este proceso lo abrió una actualización (para avisarlo en la pantalla).
var recienActualizado bool

// limpiarActualizacion borra el .viejo que dejó la actualización anterior y, si este proceso lo
// abrió una actualización (--esperar=<pid>), espera a que el launcher viejo termine de cerrarse
// (si no, el bloqueo de una sola instancia cerraría al nuevo).
func limpiarActualizacion() {
	for _, a := range os.Args[1:] {
		if p, ok := strings.CutPrefix(a, "--esperar="); ok {
			recienActualizado = true
			if pid, err := strconv.Atoi(p); err == nil {
				esperarProceso(pid)
			}
		}
	}
	if exe, err := os.Executable(); err == nil {
		os.Remove(exe + ".viejo")
	}
}
