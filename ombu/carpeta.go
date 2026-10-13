package ombu

import (
	"os"
	"path/filepath"
	"strings"
)

// sePuedeEscribir crea la carpeta y prueba escribir un archivo, como SePuedeEscribir en core.cpp.
func sePuedeEscribir(dir string) bool {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false
	}
	prueba := filepath.Join(dir, ".prueba")
	f, err := os.Create(prueba)
	if err != nil {
		return false
	}
	_, err = f.Write([]byte{'x'})
	f.Close()
	os.Remove(prueba)
	return err == nil
}

// CarpetaDatos devuelve dónde va ombuserver.mpq, con la misma regla que la DLL (core.cpp ElegirCarpeta):
// 1) la carpeta del juego, a la vista junto a ijl11.dll (decisión del usuario, 2026-10-13); 2) si ahí no se
// puede escribir (juego en Archivos de programa sin permisos), %LOCALAPPDATA%\OmbuServer.
// localAppData vacío = tomarlo del entorno.
func CarpetaDatos(carpetaJuego, localAppData string) string {
	if sePuedeEscribir(carpetaJuego) {
		return carpetaJuego
	}
	if localAppData == "" {
		localAppData = os.Getenv("LOCALAPPDATA")
	}
	if localAppData != "" {
		return filepath.Join(localAppData, NombreCarpeta)
	}
	return carpetaJuego
}

// CarpetaVieja es donde guardaban todo las versiones anteriores (%LOCALAPPDATA%\OmbuServer).
func CarpetaVieja(localAppData string) string {
	if localAppData == "" {
		localAppData = os.Getenv("LOCALAPPDATA")
	}
	if localAppData == "" {
		return ""
	}
	return filepath.Join(localAppData, NombreCarpeta)
}

// LimpiarCarpetaVieja borra lo que dejaban las versiones anteriores en %LOCALAPPDATA%\OmbuServer (voces, banners,
// versión del MPQ, logs viejos y el MPQ si los datos ya no están ahí). opciones.txt e historias.txt los migra la
// DLL al registro; la configuración del launcher la migra cargarConfig. La carpeta se borra si queda vacía.
func LimpiarCarpetaVieja(carpetaDatos, localAppData string) {
	v := CarpetaVieja(localAppData)
	if v == "" {
		return
	}
	os.RemoveAll(filepath.Join(v, "audio"))
	os.RemoveAll(filepath.Join(v, "banners"))
	for _, f := range []string{"manifest.json", ArchivoVersion, "native_hook.log", "overlay.log"} {
		os.Remove(filepath.Join(v, f))
	}
	if !strings.EqualFold(filepath.Clean(v), filepath.Clean(carpetaDatos)) {
		os.Remove(filepath.Join(v, ArchivoMPQ))
		os.Remove(filepath.Join(v, ArchivoMPQ+".tmp"))
	}
	os.Remove(v) // solo si quedó vacía
}
