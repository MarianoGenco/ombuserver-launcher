// Package chat es un cliente del chat de Battle.net (protocolo clásico BNCS) para el launcher: inicia sesión con
// una cuenta, entra a un canal y manda y recibe mensajes.
//
// Es el mismo handshake que ya usa el panel de admin (ombuserver-panel/chat.js), probado contra nuestro bnetd:
// 0x01 (protocolo BNCS) → SID_AUTH_INFO (0x50) → SID_AUTH_CHECK (0x51) → SID_LOGONRESPONSE2 (0x3A) → SID_ENTERCHAT
// (0x0A) → SID_JOINCHANNEL (0x0C). Se presenta como Diablo II LoD 1.14b con la versión 1.14.1.1 (la que manda la
// DLL de OmbuServer, mod_lanzador.cpp), así pasa el versioncheck del servidor aunque deje de aceptar la 1.14.1.0.
package chat

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Evento es lo que se manda a la ventana por cada SID_CHATEVENT (0x0F).
type Evento struct {
	Tipo      string     `json:"tipo"`    // showuser, join, leave, whisper, talk, broadcast, channel, userflags, whispersent, info, error, emote...
	Usuario   string     `json:"usuario"` // como lo manda el servidor ("Personaje*Cuenta" para jugadores de D2)
	Cuenta    string     `json:"cuenta"`  // la cuenta de Battle.net
	Texto     string     `json:"texto"`   // mensaje (o statstring en showuser/join/userflags)
	Flags     uint32     `json:"flags"`   // 0x01 Blizzard, 0x02 operador, 0x04 voz, 0x08 admin (sysop)
	Ping      uint32     `json:"ping"`
	Personaje *Personaje `json:"personaje"` // solo si el usuario está en Diablo II con un personaje
}

// Personaje sale del statstring de Diablo II ("PX2DReino,Personaje,<33 bytes>").
type Personaje struct {
	Nombre    string `json:"nombre"`
	Clase     string `json:"clase"` // ama, sor, nec, pal, bar, dru, ass
	Nivel     int    `json:"nivel"`
	Hardcore  bool   `json:"hardcore"`
	Expansion bool   `json:"expansion"`
}

var eventos = map[uint32]string{
	0x01: "showuser", 0x02: "join", 0x03: "leave", 0x04: "whisper", 0x05: "talk", 0x06: "broadcast",
	0x07: "channel", 0x09: "userflags", 0x0a: "whispersent", 0x0d: "channelfull", 0x0e: "channeldoesnotexist",
	0x0f: "channelrestricted", 0x12: "info", 0x13: "error", 0x17: "emote",
}

// Paquetes fijos del handshake (los mismos de chat.js; en AUTH_CHECK la versión va en 1.14.1.1 = 0x010e0101).
// SID_AUTH_INFO termina con el idioma y el país del cliente, y con eso PvPGN elige en qué idioma contesta
// (localize_by_country): español de Argentina, o inglés de Estados Unidos si el launcher está en inglés.
var (
	authInfoES = authInfo("53457365", "0a2c0000", "0a0c0000", "ARG", "Argentina")     // esES, es-AR
	authInfoEN = authInfo("53556e65", "09040000", "09040000", "USA", "United States") // enUS, en-US
	authCheck  = deHex("ff519100010e577b01010e01de344bb20200000000000000100000000700000029cdb2000000000027312fd62ee2fd9c5c1611cbb53bd9015f5980bb100000000a000000526504000000000063a21dc9f96d8513133ba2e2f1915d0e1c2076e747616d652e6578652030312f30332f32302030363a30343a3337203335393031323000426c697a7a426f7947616d657300")
)

func authInfoPara() []byte {
	if Ingles.Load() {
		return authInfoEN
	}
	return authInfoES
}

// Ingles: el chat se presenta como cliente en inglés (lo cambia el idioma del launcher; vale desde la próxima conexión).
var Ingles atomic.Bool

// authInfo arma SID_AUTH_INFO: protocolo 0, IX86, D2XP, versión 0x0e, idioma, IP local, zona horaria, LCID del
// sistema y del idioma, abreviatura y nombre del país.
func authInfo(idioma, lcid, lcidIdioma, pais, nombrePais string) []byte {
	d := deHex("00000000" + "36385849" + "50583244" + "0e000000" + idioma + "7f000001" + "b4000000" + lcid + lcidIdioma)
	d = append(append(d, cadena(pais)...), cadena(nombrePais)...)
	return append([]byte{0xff, 0x50, byte(len(d) + 4), 0}, d...)
}

func deHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// Cliente es una sesión de chat abierta.
type Cliente struct {
	conn    net.Conn
	escribe sync.Mutex
	cerrado chan struct{}
	unaVez  sync.Once
	Usuario string
}

// ErrLogin es el error de cuenta o contraseña.
var ErrLogin = errors.New("la cuenta o la contraseña no son correctas")

func paquete(tipo byte, datos []byte) []byte {
	p := make([]byte, 4, 4+len(datos))
	p[0], p[1] = 0xff, tipo
	binary.LittleEndian.PutUint16(p[2:], uint16(4+len(datos)))
	return append(p, datos...)
}

func cadena(s string) []byte { return append([]byte(s), 0) }

func leerPaquete(r *bufio.Reader) (byte, []byte, error) {
	var cab [4]byte
	if _, err := io.ReadFull(r, cab[:]); err != nil {
		return 0, nil, err
	}
	if cab[0] != 0xff {
		return 0, nil, fmt.Errorf("respuesta inválida del servidor")
	}
	n := int(binary.LittleEndian.Uint16(cab[2:]))
	if n < 4 {
		return 0, nil, fmt.Errorf("respuesta inválida del servidor")
	}
	datos := make([]byte, n-4)
	_, err := io.ReadFull(r, datos)
	return cab[1], datos, err
}

// handshake abre la conexión y llega hasta el SID_AUTH_CHECK aceptado. Devuelve el server token.
func handshake(direccion string) (net.Conn, *bufio.Reader, uint32, error) {
	conn, err := net.DialTimeout("tcp", direccion, 10*time.Second)
	if err != nil {
		return nil, nil, 0, errors.New("no se pudo conectar con Battle.net de OmbuServer")
	}
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	r := bufio.NewReader(conn)
	if _, err := conn.Write(append([]byte{0x01}, authInfoPara()...)); err != nil {
		conn.Close()
		return nil, nil, 0, err
	}
	var token uint32
	for {
		tipo, datos, err := leerPaquete(r)
		if err != nil {
			conn.Close()
			return nil, nil, 0, errors.New("Battle.net cortó la conexión")
		}
		switch tipo {
		case 0x25: // SID_PING
			conn.Write(paquete(0x25, datos))
		case 0x50: // SID_AUTH_INFO
			if len(datos) >= 8 {
				token = binary.LittleEndian.Uint32(datos[4:])
			}
			conn.Write(authCheck)
		case 0x51: // SID_AUTH_CHECK
			if len(datos) >= 4 && binary.LittleEndian.Uint32(datos) != 0 {
				conn.Close()
				return nil, nil, 0, errors.New("el servidor no aceptó la versión del cliente de chat")
			}
			return conn, r, token, nil
		}
	}
}

// Conectar inicia sesión, entra al chat y al canal de Diablo II, y queda leyendo en una goroutine: cada evento va
// a alEvento; cuando se corta la conexión se llama alCerrar (una sola vez).
func Conectar(direccion, usuario, clave string, alEvento func(Evento), alCerrar func(error)) (*Cliente, error) {
	conn, r, token, err := handshake(direccion)
	if err != nil {
		return nil, err
	}
	ticks := rand.Uint32()
	var toks [8]byte
	binary.LittleEndian.PutUint32(toks[0:], ticks)
	binary.LittleEndian.PutUint32(toks[4:], token)
	h1 := hashBnet([]byte(strings.ToLower(clave)))
	h2 := hashBnet(append(toks[:], h1[:]...))
	conn.Write(paquete(0x3a, append(append(toks[:], h2[:]...), cadena(usuario)...)))
	for {
		tipo, datos, err := leerPaquete(r)
		if err != nil {
			conn.Close()
			return nil, errors.New("Battle.net cortó la conexión")
		}
		if tipo == 0x25 {
			conn.Write(paquete(0x25, datos))
			continue
		}
		if tipo == 0x3a {
			res := uint32(1)
			if len(datos) >= 4 {
				res = binary.LittleEndian.Uint32(datos)
			}
			switch res {
			case 0:
			case 1, 2:
				conn.Close()
				return nil, ErrLogin
			case 6:
				conn.Close()
				return nil, errors.New("esa cuenta está cerrada")
			default:
				conn.Close()
				return nil, fmt.Errorf("Battle.net rechazó el inicio de sesión (código %d)", res)
			}
			break
		}
	}
	// SID_ENTERCHAT y el canal del producto ("Diablo II", que el servidor muestra como OmbuServer)
	conn.Write(paquete(0x0a, append(cadena(""), cadena("")...)))
	conn.Write(paquete(0x0c, append([]byte{0x01, 0, 0, 0}, cadena("Diablo II")...)))
	conn.SetDeadline(time.Time{})
	c := &Cliente{conn: conn, cerrado: make(chan struct{}), Usuario: usuario}
	go c.leer(r, alEvento, alCerrar)
	go c.mantener()
	return c, nil
}

func (c *Cliente) leer(r *bufio.Reader, alEvento func(Evento), alCerrar func(error)) {
	var err error
	for {
		var tipo byte
		var datos []byte
		tipo, datos, err = leerPaquete(r)
		if err != nil {
			break
		}
		switch tipo {
		case 0x25:
			c.escribir(paquete(0x25, datos))
		case 0x0f:
			if ev, ok := eventoDe(datos); ok && alEvento != nil {
				alEvento(ev)
			}
		}
	}
	c.Cerrar()
	if alCerrar != nil {
		alCerrar(err)
	}
}

// mantener manda SID_NULL cada minuto para que la conexión no quede ociosa.
func (c *Cliente) mantener() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-c.cerrado:
			return
		case <-t.C:
			c.escribir(paquete(0x00, nil))
		}
	}
}

func (c *Cliente) escribir(p []byte) error {
	c.escribe.Lock()
	defer c.escribe.Unlock()
	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := c.conn.Write(p)
	return err
}

// Enviar manda un mensaje o un comando ("/w cuenta hola", "/join canal", "/me ...").
func (c *Cliente) Enviar(texto string) error {
	texto = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(texto, "\r", " "), "\n", " "))
	if texto == "" {
		return nil
	}
	if len(texto) > 220 {
		texto = texto[:220]
	}
	return c.escribir(paquete(0x0e, cadena(texto)))
}

// UnirCanal cambia de canal.
func (c *Cliente) UnirCanal(nombre string) error {
	return c.escribir(paquete(0x0c, append([]byte{0x02, 0, 0, 0}, cadena(nombre)...)))
}

// Cerrar corta la sesión (se puede llamar varias veces).
func (c *Cliente) Cerrar() {
	c.unaVez.Do(func() {
		close(c.cerrado)
		c.conn.Close()
	})
}

// eventoDe interpreta un SID_CHATEVENT: id, flags, ping, ip, cuenta, regauth (6 DWORD), usuario y texto.
func eventoDe(d []byte) (Evento, bool) {
	if len(d) < 24 {
		return Evento{}, false
	}
	id := binary.LittleEndian.Uint32(d[0:])
	ev := Evento{Flags: binary.LittleEndian.Uint32(d[4:]), Ping: binary.LittleEndian.Uint32(d[8:])}
	resto := d[24:]
	usuario, resto := cortarCadena(resto)
	texto, _ := cortarCadena(resto)
	ev.Usuario, ev.Texto = string(usuario), string(texto)
	if t, ok := eventos[id]; ok {
		ev.Tipo = t
	} else {
		ev.Tipo = fmt.Sprintf("0x%x", id)
	}
	ev.Cuenta = ev.Usuario
	if i := strings.IndexByte(ev.Usuario, '*'); i >= 0 {
		ev.Cuenta = ev.Usuario[i+1:]
	}
	switch ev.Tipo {
	case "showuser", "join", "userflags":
		ev.Personaje = PersonajeDe(ev.Texto)
	}
	return ev, true
}

func cortarCadena(b []byte) ([]byte, []byte) {
	i := bytes.IndexByte(b, 0)
	if i < 0 {
		return b, nil
	}
	return b[:i], b[i+1:]
}

// PersonajeDe interpreta el statstring de Diablo II: producto al revés ("PX2D" o "VD2D"), "Reino,Personaje," y 33
// bytes: [13] clase (1 amazona ... 7 asesina), [25] nivel, [26] banderas (0x04 hardcore, 0x20 expansión).
// Sin personaje (solo el producto) devuelve nil.
func PersonajeDe(s string) *Personaje {
	if len(s) < 4 || (s[:4] != "PX2D" && s[:4] != "VD2D") {
		return nil
	}
	partes := strings.SplitN(s[4:], ",", 3)
	if len(partes) < 3 || partes[1] == "" || len(partes[2]) < 27 {
		return nil
	}
	st := []byte(partes[2])
	clases := map[byte]string{1: "ama", 2: "sor", 3: "nec", 4: "pal", 5: "bar", 6: "dru", 7: "ass"}
	p := &Personaje{Nombre: partes[1], Clase: clases[st[13]], Nivel: int(st[25])}
	if st[26] != 0xff {
		p.Hardcore = st[26]&0x04 != 0
		p.Expansion = st[26]&0x20 != 0
	}
	if p.Nivel == 0xff || p.Nivel < 1 || p.Nivel > 99 {
		p.Nivel = 0
	}
	return p
}
