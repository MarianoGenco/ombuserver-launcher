package ombu

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMezclarGatewaysVacio(t *testing.T) {
	got := MezclarGateways(nil, "203.0.113.10", "-3", "OmbuServer")
	want := []string{"1002", "01", "203.0.113.10", "-3", "OmbuServer"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v != %v", got, want)
	}
}

func TestMezclarGatewaysConservaLosDelJugador(t *testing.T) {
	actual := []string{"1002", "01", "uswest.battle.net", "8", "U.S. West", "190.0.162.136", "-3", "OmbuServer"}
	got := MezclarGateways(actual, "203.0.113.10", "-3", "OmbuServer")
	want := []string{"1002", "02", "uswest.battle.net", "8", "U.S. West", "203.0.113.10", "-3", "OmbuServer"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v != %v", got, want)
	}
	if n, ok := IndiceElegido(got); !ok || n != "OmbuServer" {
		t.Fatalf("elegido %q %v", n, ok)
	}
}

func TestCarpetaDatos(t *testing.T) {
	juego, app := t.TempDir(), t.TempDir()
	if d := CarpetaDatos(juego, app); d != juego {
		t.Fatalf("con la carpeta del juego escribible: %s", d)
	}
	// carpeta del juego donde no se puede escribir (un archivo en su lugar): AppData
	malo := filepath.Join(t.TempDir(), "archivo")
	os.WriteFile(malo, []byte("x"), 0644)
	if d := CarpetaDatos(malo, app); d != filepath.Join(app, "OmbuServer") {
		t.Fatalf("sin carpeta del juego: %s", d)
	}
}

func servidor(t *testing.T, archivos map[string][]byte) *Cliente {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := archivos[r.URL.Path[1:]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	t.Cleanup(s.Close)
	return &Cliente{Base: s.URL + "/", HTTP: s.Client()}
}

func TestSincronizarMPQ(t *testing.T) {
	mpq := []byte("contenido del mundo 6")
	suma := md5.Sum(mpq)
	ver := hex.EncodeToString(suma[:])
	c := servidor(t, map[string][]byte{ArchivoMPQ: mpq, ArchivoVersion: []byte(ver + "\n")})
	datos := t.TempDir()
	cambio, err := c.SincronizarMPQ(datos, nil)
	if err != nil || !cambio {
		t.Fatalf("primera vez: %v %v", cambio, err)
	}
	if VersionLocalMPQ(datos) != ver {
		t.Fatal("el MPQ bajado no tiene la versión publicada")
	}
	cambio, err = c.SincronizarMPQ(datos, nil)
	if err != nil || cambio {
		t.Fatalf("segunda vez no tenía que bajar: %v %v", cambio, err)
	}
}

func TestSincronizarMPQRechazaCorrupto(t *testing.T) {
	c := servidor(t, map[string][]byte{ArchivoMPQ: []byte("roto"), ArchivoVersion: []byte("0123456789abcdef0123456789abcdef")})
	datos := t.TempDir()
	if _, err := c.SincronizarMPQ(datos, nil); err == nil {
		t.Fatal("tenía que rechazar un MPQ que no coincide con la versión")
	}
	if _, err := os.Stat(filepath.Join(datos, ArchivoMPQ)); !os.IsNotExist(err) {
		t.Fatal("dejó un MPQ corrupto")
	}
}

func TestClienteSoloIjl11(t *testing.T) {
	dll := []byte("MZ ijl11 de OmbuServer")
	crc := fmt.Sprintf("%08x", crc32.ChecksumIEEE(dll))
	man := fmt.Sprintf(`{"files":[{"name":"ijl11.dll","crc":"%s","content_length":%d},`+
		`{"name":"BH.dll","crc":"00000000","content_length":1}]}`, crc, len(dll))
	c := servidor(t, map[string][]byte{ManifiestoCliente: []byte(man), CarpetaCliente + "ijl11.dll": dll})
	juego := t.TempDir()
	acc, _, err := c.PendientesCliente(juego)
	if err == nil {
		t.Fatal("tenía que avisar que BH.dll no está permitido")
	}
	if len(acc) != 1 || acc[0].Archivo.Name != "ijl11.dll" {
		t.Fatalf("acciones: %+v", acc)
	}
	if err := c.AplicarCliente(juego, acc, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := CRC32Archivo(filepath.Join(juego, "ijl11.dll")); got != crc {
		t.Fatalf("crc %s != %s", got, crc)
	}
	if _, err := os.Stat(filepath.Join(juego, "BH.dll")); !os.IsNotExist(err) {
		t.Fatal("escribió un archivo no permitido")
	}
	acc, _, _ = c.PendientesCliente(juego)
	if len(acc) != 0 {
		t.Fatalf("ya estaba al día: %+v", acc)
	}
}
