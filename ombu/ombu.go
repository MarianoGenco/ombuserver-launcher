// Package ombu es el núcleo del launcher de OmbuServer: todo lo que no es interfaz.
//
// No depende de Qt (se compila y se prueba con Go solo) y la interfaz (bridge/ + qml/) lo usa
// a través del servicio d2. Reglas del proyecto que respeta (memoria project_decisiones_mod):
//   - En la carpeta del juego van ijl11.dll y ombuserver.mpq (con las voces adentro), a la vista
//     (decisión del usuario, 2026-10-13). Si ahí no se puede escribir, el MPQ va a %LOCALAPPDATA%\OmbuServer.
//     Es la misma regla que OmbuServer-dll/src/core.cpp (CacheDir).
//   - Las opciones (de la DLL y del launcher) van al registro: HKCU\Software\OmbuServer.
//   - Los secretos quedan en el servidor: el launcher solo baja contenido público.
package ombu

// Direcciones del servidor. Acá va la dirección de TU servidor: en el código quedan de ejemplo y las reales se ponen
// al compilar, sin tocar el código:
//
//	wails build ... -ldflags "-X ombuserver-launcher/ombu.URLContenido=http://<web>/contenido/
//	                          -X ombuserver-launcher/ombu.IPRealm=<ip del juego> ..."
//
// (ver también URLDescargas en instalar.go y urlVoz en voz.go).
var (
	// URLContenido: el contenido que publica el servidor web (novedades, launcher, MPQ, voces).
	// Es la misma base que usa la DLL del cliente.
	URLContenido = "http://IP-DEL-SERVIDOR-WEB/contenido/"

	// IPRealm: el gateway de Battle.net que se escribe en el registro (Battle.net/PvPGN, puerto 6112).
	IPRealm = "IP-DEL-SERVIDOR-DE-JUEGO"
)

const (
	// NombreRealm: el nombre del gateway en la lista de Battle.net del juego.
	NombreRealm = "OmbuServer"
	ZonaRealm   = "-3" // huso horario del gateway (Argentina)

	// SHA1GameExe114b es el Game.exe 1.14b para el que están hechos todos los parches de la DLL
	// (direcciones fijas: INGENIERIA_INVERSA/Game114b.exe.md). Ojo: NO es el "1.14b" de la lista
	// del launcher de Slash (11e94026...); es otra compilación (3 590 120 bytes, 01/03/2020).
	SHA1GameExe114b = "07786750cb1dc6014c68649df3d1955c05a0c81c"

	// NombreCarpeta es la carpeta de datos (en AppData o dentro de la carpeta del juego).
	NombreCarpeta = "OmbuServer"

	// ArchivoMPQ y ArchivoVersion: el contenido del Mundo 6. La versión es el md5 del MPQ
	// (MOD/construir.py la escribe; la DLL la compara en mod_mpq.cpp).
	ArchivoMPQ     = "ombuserver.mpq"
	ArchivoVersion = "ombuserver.ver"

	// ManifiestoCliente: archivos de la carpeta del juego, en el formato del launcher de Slash
	// ({"files":[{"name","crc","content_length","ignore_crc","deprecated"}]}, CRC32 IEEE en hex).
	// Se publica en /contenido/cliente/manifest.json y los archivos en /contenido/cliente/<nombre>.
	ManifiestoCliente = "cliente/manifest.json"
	CarpetaCliente    = "cliente/"
)

// ArchivosPermitidos son los únicos nombres que el launcher acepta escribir en la carpeta del
// juego, aunque el manifiesto publicado diga otra cosa ("sin extras para el jugador").
var ArchivosPermitidos = map[string]bool{
	"ijl11.dll": true,
}

// BanderasPorDefecto: ventana (el banner del lobby usa el motor GDI) y directo a Battle.net.
var BanderasPorDefecto = []string{"-w", "-skiptobnet"}
