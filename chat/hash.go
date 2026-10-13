package chat

import "encoding/binary"

func rotl(x uint32, n uint) uint32 { n &= 31; return x<<n | x>>(32-n) }

// hashBnet es el "broken SHA-1" (X-SHA1) de Battle.net: sin relleno ni largo al final, palabras en little endian y
// una rotación al revés. Es el mismo de ombuserver-panel/chat.js (bnetHash), devuelto como 20 bytes little endian.
func hashBnet(datos []byte) [20]byte {
	h := [5]uint32{0x67452301, 0xefcdab89, 0x98badcfe, 0x10325476, 0xc3d2e1f0}
	trozos := [][]byte{}
	for i := 0; i < len(datos); i += 64 {
		f := i + 64
		if f > len(datos) {
			f = len(datos)
		}
		trozos = append(trozos, datos[i:f])
	}
	if len(trozos) == 0 {
		trozos = append(trozos, nil)
	}
	for _, t := range trozos {
		var w [80]uint32
		for i := 0; i < 16; i++ {
			var p uint32
			for b := 0; b < 4; b++ {
				if i*4+b < len(t) {
					p |= uint32(t[i*4+b]) << (8 * b)
				}
			}
			w[i] = p
		}
		for i := 0; i < 64; i++ {
			w[i+16] = rotl(1, uint(w[i]^w[i+8]^w[i+2]^w[i+13]))
		}
		a, b, c, d, e := h[0], h[1], h[2], h[3], h[4]
		var g uint32
		for i := 0; i < 20; i++ {
			g = w[i] + rotl(a, 5) + e + ((b & c) | (^b & d)) + 0x5a827999
			e, d, c, b, a = d, c, rotl(b, 30), a, g
		}
		for i := 20; i < 40; i++ {
			g = (d ^ c ^ b) + e + rotl(g, 5) + w[i] + 0x6ed9eba1
			e, d, c, b, a = d, c, rotl(b, 30), a, g
		}
		for i := 40; i < 60; i++ {
			g = w[i] + rotl(g, 5) + e + ((c & b) | (d & c) | (d & b)) - 0x70e44324
			e, d, c, b, a = d, c, rotl(b, 30), a, g
		}
		for i := 60; i < 80; i++ {
			g = (d ^ c ^ b) + e + rotl(g, 5) + w[i] - 0x359d3e2a
			e, d, c, b, a = d, c, rotl(b, 30), a, g
		}
		h[0] += g
		h[1] += b
		h[2] += c
		h[3] += d
		h[4] += e
	}
	var out [20]byte
	for i, v := range h {
		binary.LittleEndian.PutUint32(out[i*4:], v)
	}
	return out
}
