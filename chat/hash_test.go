package chat

import (
	"encoding/hex"
	"strings"
	"testing"
)

// Valores del bnetHash de ombuserver-panel/chat.js (probado contra el servidor).
func TestHashBnet(t *testing.T) {
	casos := map[string]string{}
	for _, l := range strings.Split(strings.TrimSpace(esperado), "\n") {
		p := strings.Fields(l)
		casos[p[0]] = p[1]
	}
	for entrada, hex1 := range map[string]string{"": casos["\"\""], "password": casos["\"password\""], "hola123": casos["\"hola123\""]} {
		h := hashBnet([]byte(entrada))
		if got := hex.EncodeToString(h[:]); got != hex1 {
			t.Errorf("%q: %s, se esperaba %s", entrada, got, hex1)
		}
	}
	h := hashBnet([]byte(strings.Repeat("x", 70)))
	if got := hex.EncodeToString(h[:]); got != casos["\"xxxxxxxx\""] {
		t.Errorf("70 x: %s, se esperaba %s", got, casos["\"xxxxxxxx\""])
	}
}

func TestPersonajeDe(t *testing.T) {
	st := make([]byte, 33)
	for i := range st {
		st[i] = 0x80
	}
	st[13], st[25], st[26] = 4, 85, 0x20|0x80
	p := PersonajeDe("PX2DOmbuServer,hechitest," + string(st))
	if p == nil || p.Nombre != "hechitest" || p.Clase != "pal" || p.Nivel != 85 || !p.Expansion || p.Hardcore {
		t.Fatalf("%+v", p)
	}
	if PersonajeDe("PX2D") != nil {
		t.Fatal("sin personaje debería ser nil")
	}
}

const esperado = `
"" eea03a4d5a1d2694576f4a5860998d6b80c64615
"password" ecc80d1d76e758c0b9da8c25ff106aff8e242916
"hola123" 34fb5d05b52171ee3f7fdd57abcb52904b38db80
"xxxxxxxx" 6eff89fc62522f3e827501198203937af1aa83b0
`
