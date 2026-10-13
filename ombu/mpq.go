package ombu

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Lector mínimo de MPQ (formato v1, el de Diablo II), para sacar archivos de ombuserver.mpq: las tablas de hash y de
// bloques cifradas, archivos cifrados con la clave de su nombre, y sectores sin comprimir o con zlib (así lo arma
// MOD/mpq.py crear). El MPQ no tiene lista de archivos: se busca por el nombre exacto, como hace el juego.

const (
	mpqExiste      = 0x80000000
	mpqUnSolo      = 0x01000000
	mpqCRCSector   = 0x04000000
	mpqCifrado     = 0x00010000
	mpqClaveFija   = 0x00020000
	mpqComprimido  = 0x00000200
	mpqImplode     = 0x00000100
	mpqSinBloque   = 0xFFFFFFFF
	mpqCompZlib    = 0x02
	mpqTopeArchivo = 64 << 20
)

var tablaCripto = func() (t [0x500]uint32) {
	semilla := uint32(0x00100001)
	for i := 0; i < 0x100; i++ {
		for j, k := i, 0; k < 5; j, k = j+0x100, k+1 {
			semilla = (semilla*125 + 3) % 0x2AAAAB
			alto := (semilla & 0xFFFF) << 16
			semilla = (semilla*125 + 3) % 0x2AAAAB
			t[j] = alto | (semilla & 0xFFFF)
		}
	}
	return
}()

func hashMPQ(s string, tipo uint32) uint32 {
	s1, s2 := uint32(0x7FED7FED), uint32(0xEEEEEEEE)
	for _, c := range []byte(strings.ToUpper(strings.ReplaceAll(s, "/", `\`))) {
		s1 = tablaCripto[tipo<<8+uint32(c)] ^ (s1 + s2)
		s2 = uint32(c) + s1 + s2 + (s2 << 5) + 3
	}
	return s1
}

func descifrar(d []uint32, clave uint32) {
	s := uint32(0xEEEEEEEE)
	for i := range d {
		s += tablaCripto[0x400+(clave&0xFF)]
		v := d[i] ^ (clave + s)
		clave = ((^clave << 21) + 0x11111111) | (clave >> 11)
		s = v + s + (s << 5) + 3
		d[i] = v
	}
}

// descifrarBytes descifra de a 4 bytes (lo que sobra al final queda como está).
func descifrarBytes(b []byte, clave uint32) {
	n := len(b) / 4
	d := make([]uint32, n)
	for i := range d {
		d[i] = binary.LittleEndian.Uint32(b[i*4:])
	}
	descifrar(d, clave)
	for i, v := range d {
		binary.LittleEndian.PutUint32(b[i*4:], v)
	}
}

type entradaHash struct{ a, b, bloque uint32 }
type entradaBloque struct{ pos, comprimido, tamano, banderas uint32 }

// MPQ abierto para leer.
type MPQ struct {
	f       *os.File
	base    int64
	sector  int64
	hashes  []entradaHash
	bloques []entradaBloque
}

// AbrirMPQ abre el archivo y lee sus tablas. Hay que cerrarlo (Cerrar) para no trabar la actualización del MPQ.
func AbrirMPQ(ruta string) (*MPQ, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return nil, err
	}
	m := &MPQ{f: f}
	if err := m.leerTablas(); err != nil {
		f.Close()
		return nil, fmt.Errorf("%s: %v", ruta, err)
	}
	return m, nil
}

// Cerrar libera el archivo.
func (m *MPQ) Cerrar() error { return m.f.Close() }

func (m *MPQ) leerTablas() error {
	var cab [32]byte
	for base := int64(0); ; base += 512 { // la cabecera puede estar en cualquier múltiplo de 512
		if _, err := m.f.ReadAt(cab[:], base); err != nil {
			return errors.New("no es un MPQ")
		}
		if string(cab[:4]) == "MPQ\x1a" {
			m.base = base
			break
		}
		if base > 1<<20 {
			return errors.New("no es un MPQ")
		}
	}
	m.sector = 512 << binary.LittleEndian.Uint16(cab[14:])
	posHash, posBloque := int64(binary.LittleEndian.Uint32(cab[16:])), int64(binary.LittleEndian.Uint32(cab[20:]))
	nHash, nBloque := binary.LittleEndian.Uint32(cab[24:]), binary.LittleEndian.Uint32(cab[28:])
	if nHash == 0 || nHash > 1<<20 || nBloque > 1<<20 || nHash&(nHash-1) != 0 {
		return errors.New("tablas del MPQ inválidas")
	}
	h, err := m.leerTabla(posHash, nHash, hashMPQ("(hash table)", 3))
	if err != nil {
		return err
	}
	b, err := m.leerTabla(posBloque, nBloque, hashMPQ("(block table)", 3))
	if err != nil {
		return err
	}
	for i := 0; i+3 < len(h); i += 4 {
		m.hashes = append(m.hashes, entradaHash{h[i], h[i+1], h[i+3]})
	}
	for i := 0; i+3 < len(b); i += 4 {
		m.bloques = append(m.bloques, entradaBloque{b[i], b[i+1], b[i+2], b[i+3]})
	}
	return nil
}

func (m *MPQ) leerTabla(pos int64, n, clave uint32) ([]uint32, error) {
	b := make([]byte, int(n)*16)
	if _, err := m.f.ReadAt(b, m.base+pos); err != nil {
		return nil, err
	}
	d := make([]uint32, int(n)*4)
	for i := range d {
		d[i] = binary.LittleEndian.Uint32(b[i*4:])
	}
	descifrar(d, clave)
	return d, nil
}

// Leer devuelve el contenido del archivo interno (por ejemplo `data\ombu\launcher\chat\pasto.png`).
// Si no está, el error es os.ErrNotExist.
func (m *MPQ) Leer(nombre string) ([]byte, error) {
	n := uint32(len(m.hashes))
	a, b := hashMPQ(nombre, 1), hashMPQ(nombre, 2)
	inicio := hashMPQ(nombre, 0) & (n - 1)
	for i, k := inicio, uint32(0); k < n; i, k = (i+1)&(n-1), k+1 {
		e := m.hashes[i]
		if e.bloque == mpqSinBloque {
			break
		}
		if e.a == a && e.b == b && int(e.bloque) < len(m.bloques) {
			return m.leerBloque(m.bloques[e.bloque], nombre)
		}
	}
	return nil, os.ErrNotExist
}

func (m *MPQ) leerBloque(bl entradaBloque, nombre string) ([]byte, error) {
	if bl.banderas&mpqExiste == 0 {
		return nil, os.ErrNotExist
	}
	if bl.tamano > mpqTopeArchivo || bl.comprimido > mpqTopeArchivo {
		return nil, errors.New(nombre + ": demasiado grande")
	}
	if bl.banderas&mpqImplode != 0 {
		return nil, errors.New(nombre + ": compresión PKWARE no soportada")
	}
	crudo := make([]byte, bl.comprimido)
	if _, err := m.f.ReadAt(crudo, m.base+int64(bl.pos)); err != nil && err != io.EOF {
		return nil, err
	}
	var clave uint32
	if bl.banderas&mpqCifrado != 0 {
		corto := nombre[strings.LastIndexAny(nombre, `\/`)+1:]
		clave = hashMPQ(corto, 3)
		if bl.banderas&mpqClaveFija != 0 {
			clave = (clave + bl.pos) ^ bl.tamano
		}
	}
	comprimido := bl.banderas&mpqComprimido != 0
	if bl.banderas&mpqUnSolo != 0 {
		if clave != 0 {
			descifrarBytes(crudo, clave)
		}
		return descomprimir(crudo, int(bl.tamano), comprimido)
	}
	nSect := (int64(bl.tamano) + m.sector - 1) / m.sector
	salida := make([]byte, 0, bl.tamano)
	if !comprimido { // sectores seguidos, sin tabla
		for i := int64(0); i < nSect; i++ {
			desde, hasta := i*m.sector, min((i+1)*m.sector, int64(bl.tamano))
			if hasta > int64(len(crudo)) {
				return nil, errors.New(nombre + ": archivo cortado")
			}
			s := crudo[desde:hasta]
			if clave != 0 {
				descifrarBytes(s, clave+uint32(i))
			}
			salida = append(salida, s...)
		}
		return salida, nil
	}
	nOff := nSect + 1
	if bl.banderas&mpqCRCSector != 0 {
		nOff++
	}
	if nOff*4 > int64(len(crudo)) {
		return nil, errors.New(nombre + ": tabla de sectores inválida")
	}
	tabla := crudo[:nOff*4]
	if clave != 0 {
		tabla = append([]byte(nil), tabla...)
		descifrarBytes(tabla, clave-1)
	}
	for i := int64(0); i < nSect; i++ {
		desde, hasta := int64(binary.LittleEndian.Uint32(tabla[i*4:])), int64(binary.LittleEndian.Uint32(tabla[i*4+4:]))
		if desde > hasta || hasta > int64(len(crudo)) {
			return nil, errors.New(nombre + ": sector inválido")
		}
		s := crudo[desde:hasta]
		if clave != 0 {
			descifrarBytes(s, clave+uint32(i))
		}
		esperado := int(min(m.sector, int64(bl.tamano)-i*m.sector))
		d, err := descomprimir(s, esperado, true)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", nombre, err)
		}
		salida = append(salida, d...)
	}
	if len(salida) != int(bl.tamano) {
		return nil, errors.New(nombre + ": tamaño distinto al esperado")
	}
	return salida, nil
}

// descomprimir un sector: si ocupa lo mismo que el original va sin comprimir; si no, el primer byte dice la
// compresión (solo zlib).
func descomprimir(s []byte, esperado int, comprimido bool) ([]byte, error) {
	if !comprimido || len(s) >= esperado {
		return s[:min(len(s), esperado)], nil
	}
	if len(s) == 0 || s[0] != mpqCompZlib {
		return nil, fmt.Errorf("compresión no soportada (0x%02x)", s[0])
	}
	z, err := zlib.NewReader(bytes.NewReader(s[1:]))
	if err != nil {
		return nil, err
	}
	defer z.Close()
	d, err := io.ReadAll(io.LimitReader(z, int64(esperado)+1))
	if err != nil {
		return nil, err
	}
	if len(d) != esperado {
		return nil, errors.New("sector de tamaño distinto al esperado")
	}
	return d, nil
}
