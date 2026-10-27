package ombu

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Progreso informa bytes bajados sobre el total (total 0 = desconocido).
type Progreso func(hechos, total int64)

// Cliente baja el contenido público de OmbuServer.
type Cliente struct {
	Base string
	HTTP *http.Client
}

// NuevoCliente con la base publicada y un tiempo límite generoso (el MPQ pesa medio mega hoy,
// pero va a crecer con los escenarios del Mundo 6).
func NuevoCliente() *Cliente {
	return &Cliente{Base: URLContenido, HTTP: &http.Client{Timeout: 10 * time.Minute}}
}

func (c *Cliente) get(ruta string) (*http.Response, error) {
	r, err := c.HTTP.Get(c.Base + ruta)
	if err != nil {
		return nil, err
	}
	if r.StatusCode < 200 || r.StatusCode > 299 {
		r.Body.Close()
		return nil, fmt.Errorf("%s: estado %d", ruta, r.StatusCode)
	}
	return r, nil
}

// Texto baja un archivo chico de texto (versiones, manifiestos).
func (c *Cliente) Texto(ruta string) (string, error) {
	r, err := c.get(ruta)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err == nil && r.StatusCode == http.StatusOK {
		textosMu.Lock()
		textos[ruta] = textoGuardado{string(b), time.Now()}
		textosMu.Unlock()
	}
	return string(b), err
}

// Lo ultimo que contesto el servidor por cada ruta (manifiesto de la DLL, version del MPQ). JUGAR lo reusa si es
// reciente: el launcher ya lo consulta al abrirse y al volver a la ventana, y cada consulta por HTTPS suma ~0,6 s.
type textoGuardado struct {
	texto string
	hora  time.Time
}

var (
	textosMu sync.Mutex
	textos   = map[string]textoGuardado{}
)

// textoReciente devuelve la ultima respuesta si tiene menos de maxEdad; si no, la pide de nuevo.
func (c *Cliente) textoReciente(ruta string, maxEdad time.Duration) (string, error) {
	if maxEdad > 0 {
		textosMu.Lock()
		g, ok := textos[ruta]
		textosMu.Unlock()
		if ok && time.Since(g.hora) < maxEdad {
			return g.texto, nil
		}
	}
	return c.Texto(ruta)
}

// Bajar guarda la ruta remota en destino y devuelve tamaño, md5 y crc32 (hex) de lo bajado.
func (c *Cliente) Bajar(ruta, destino string, p Progreso) (int64, string, string, error) {
	r, err := c.get(ruta)
	if err != nil {
		return 0, "", "", err
	}
	defer r.Body.Close()
	if err := os.MkdirAll(filepath.Dir(destino), 0755); err != nil {
		return 0, "", "", err
	}
	f, err := os.Create(destino)
	if err != nil {
		return 0, "", "", err
	}
	hm, hc := md5.New(), crc32.NewIEEE()
	var n int64
	buf := make([]byte, 64*1024)
	for {
		k, er := r.Body.Read(buf)
		if k > 0 {
			if _, ew := f.Write(buf[:k]); ew != nil {
				f.Close()
				return n, "", "", ew
			}
			hm.Write(buf[:k])
			hc.Write(buf[:k])
			n += int64(k)
			if p != nil {
				p(n, r.ContentLength)
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			f.Close()
			return n, "", "", er
		}
	}
	if err := f.Close(); err != nil {
		return n, "", "", err
	}
	return n, hex.EncodeToString(hm.Sum(nil)), hex.EncodeToString(hc.Sum(nil)), nil
}

// ErrEnUso: el archivo está abierto por un Game.exe (hay que cerrar el juego o reintentar).
var ErrEnUso = errors.New("el archivo está en uso: cerrá Diablo II y volvé a intentar")

func limpiar(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// VersionLocalMPQ es el md5 del MPQ que hay en la carpeta de datos ("" si no hay). Ya no se guarda un .ver
// aparte: la versión es el propio archivo (la DLL compara igual, mod_mpq.cpp Md5Archivo).
func VersionLocalMPQ(carpetaDatos string) string {
	f, err := os.Open(filepath.Join(carpetaDatos, ArchivoMPQ))
	if err != nil {
		return ""
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// VersionRemotaMPQ es el md5 publicado del MPQ.
func (c *Cliente) VersionRemotaMPQ() (string, error) {
	return c.versionRemotaMPQ(0)
}

func (c *Cliente) versionRemotaMPQ(maxEdad time.Duration) (string, error) {
	v, err := c.textoReciente(ArchivoVersion, maxEdad)
	return limpiar(v), err
}

// SincronizarMPQ deja en la carpeta de datos el ombuserver.mpq publicado (el servidor publica su md5 en
// ombuserver.ver). A diferencia de la DLL, verifica el md5
// de lo bajado antes de reemplazar, y si el MPQ está abierto por el juego no lo pisa a medias.
// Devuelve si cambió algo.
func (c *Cliente) SincronizarMPQ(carpetaDatos string, p Progreso) (bool, error) {
	return c.SincronizarMPQReciente(carpetaDatos, p, 0)
}

// SincronizarMPQReciente es SincronizarMPQ pero usa la version publicada que se consulto hace menos de maxEdad.
func (c *Cliente) SincronizarMPQReciente(carpetaDatos string, p Progreso, maxEdad time.Duration) (bool, error) {
	mpqMu.Lock() // JUGAR y el chat (figuras) pueden pedirlo a la vez: uno solo baja
	defer mpqMu.Unlock()
	remota, err := c.versionRemotaMPQ(maxEdad)
	if err != nil {
		return false, err
	}
	if len(remota) != 32 {
		return false, fmt.Errorf("versión publicada inválida: %q", remota)
	}
	if VersionLocalMPQ(carpetaDatos) == remota {
		return false, nil
	}
	final := filepath.Join(carpetaDatos, ArchivoMPQ)
	tmp := final + ".tmp"
	_, suma, _, err := c.Bajar(ArchivoMPQ, tmp, p)
	if err != nil {
		os.Remove(tmp)
		return false, err
	}
	if suma != remota {
		os.Remove(tmp)
		return false, fmt.Errorf("el MPQ bajado no coincide con la versión publicada (%s != %s)", suma, remota)
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return false, ErrEnUso
	}
	return true, nil
}

var mpqMu sync.Mutex

// ArchivoManifiesto es una fila del manifiesto (formato del launcher de Slash).
type ArchivoManifiesto struct {
	Name          string `json:"name"`
	CRC           string `json:"crc"`
	ContentLength int64  `json:"content_length"`
	IgnoreCRC     bool   `json:"ignore_crc"`
	Deprecated    bool   `json:"deprecated"`
}

// Manifiesto de los archivos de la carpeta del juego.
type Manifiesto struct {
	Files []ArchivoManifiesto `json:"files"`
}

// Accion pendiente sobre la carpeta del juego.
type Accion struct {
	Archivo  ArchivoManifiesto
	Borrar   bool
	CRCLocal string
}

// CRC32Archivo calcula el CRC32 IEEE (el mismo polinomio 0xedb88320 que usa Slash) en hex.
func CRC32Archivo(ruta string) (string, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := crc32.NewIEEE()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// PendientesCliente compara el manifiesto publicado con la carpeta del juego. Solo considera los
// nombres de ArchivosPermitidos (el resto del manifiesto se ignora y se avisa en el error).
func (c *Cliente) PendientesCliente(carpetaJuego string) ([]Accion, int64, error) {
	return c.PendientesClienteReciente(carpetaJuego, 0)
}

// PendientesClienteReciente compara contra el manifiesto consultado hace menos de maxEdad (0 = pedirlo siempre).
// Los CRC de los archivos locales se calculan siempre.
func (c *Cliente) PendientesClienteReciente(carpetaJuego string, maxEdad time.Duration) ([]Accion, int64, error) {
	t, err := c.textoReciente(ManifiestoCliente, maxEdad)
	if err != nil {
		return nil, 0, err
	}
	var m Manifiesto
	if err := json.Unmarshal([]byte(t), &m); err != nil {
		return nil, 0, fmt.Errorf("manifiesto del cliente: %v", err)
	}
	var acciones []Accion
	var total int64
	var raros []string
	for _, a := range m.Files {
		if !ArchivosPermitidos[strings.ToLower(a.Name)] || strings.ContainsAny(a.Name, `/\:`) {
			raros = append(raros, a.Name)
			continue
		}
		local := filepath.Join(carpetaJuego, a.Name)
		crc, err := CRC32Archivo(local)
		existe := err == nil
		if err != nil && !os.IsNotExist(err) {
			return nil, 0, err
		}
		switch {
		case a.Deprecated:
			if existe {
				acciones = append(acciones, Accion{Archivo: a, Borrar: true, CRCLocal: crc})
			}
		case !existe || (!a.IgnoreCRC && limpiar(crc) != limpiar(a.CRC)):
			acciones = append(acciones, Accion{Archivo: a, CRCLocal: crc})
			total += a.ContentLength
		}
	}
	if len(raros) > 0 {
		return acciones, total, fmt.Errorf("el manifiesto trae archivos no permitidos en la carpeta del juego: %v", raros)
	}
	return acciones, total, nil
}

// reemplazar pone tmp en lugar de final. Si final es una DLL cargada por un Game.exe abierto,
// Windows no deja pisarla pero sí renombrarla: se la mueve a <final>.viejo-<hora> y se copia la
// nueva (memoria reference_copiar_dll: "renombrar la cargada y copiar la nueva").
func reemplazar(tmp, final string) error {
	if err := os.Rename(tmp, final); err == nil {
		return nil
	}
	viejo := fmt.Sprintf("%s.viejo-%s", final, time.Now().Format("20060102-150405"))
	if err := os.Rename(final, viejo); err != nil {
		return ErrEnUso
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Rename(viejo, final)
		return ErrEnUso
	}
	return nil
}

// AplicarCliente baja/borra lo pendiente en la carpeta del juego, verificando el CRC de cada
// archivo antes de ponerlo en su lugar.
func (c *Cliente) AplicarCliente(carpetaJuego string, acciones []Accion, p Progreso) error {
	var total, hechos int64
	for _, a := range acciones {
		if !a.Borrar {
			total += a.Archivo.ContentLength
		}
	}
	for _, a := range acciones {
		final := filepath.Join(carpetaJuego, a.Archivo.Name)
		if a.Borrar {
			if err := os.Remove(final); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		tmp := final + ".tmp"
		base := hechos
		n, _, crc, err := c.Bajar(CarpetaCliente+a.Archivo.Name, tmp, func(h, _ int64) {
			if p != nil {
				p(base+h, total)
			}
		})
		if err != nil {
			os.Remove(tmp)
			return err
		}
		hechos += n
		if !a.Archivo.IgnoreCRC && limpiar(crc) != limpiar(a.Archivo.CRC) {
			os.Remove(tmp)
			return fmt.Errorf("%s: CRC bajado %s distinto del publicado %s", a.Archivo.Name, crc, a.Archivo.CRC)
		}
		if err := reemplazar(tmp, final); err != nil {
			os.Remove(tmp)
			return err
		}
	}
	return nil
}
