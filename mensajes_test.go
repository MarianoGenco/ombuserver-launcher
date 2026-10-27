package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ombuserver-launcher/chat"
)

// API de mensajes simulada: un token por entrada; "vencer" invalida el token actual (como si hubiera vencido).
func TestMensajesSesion(t *testing.T) {
	entradas, token := 0, ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/trades-api/mensajes/entrar":
			var p map[string]string
			json.NewDecoder(r.Body).Decode(&p)
			if p["cuenta"] != "arena" || p["hash"] != chat.HashClaveHex("secreta") {
				w.WriteHeader(401)
				w.Write([]byte(`{"ok":false,"error":"cuenta o contraseña incorrectas"}`))
				return
			}
			entradas++
			token = strings.Repeat("ab", 32)[:62] + string(rune('a'+entradas)) + "0"
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "token": token})
		case "/trades-api/mensajes/conversaciones":
			if r.Header.Get("Authorization") != "Bearer "+token || token == "" {
				w.WriteHeader(401)
				w.Write([]byte(`{"ok":false,"error":"sesión vencida"}`))
				return
			}
			w.Write([]byte(`{"ok":true,"sinLeer":1,"conversaciones":[]}`))
		case "/trades-api/mensajes/enviar":
			var p map[string]any
			json.NewDecoder(r.Body).Decode(&p)
			if p["para"] != "Agos" || p["item"].(map[string]any)["n"] != "Shako" {
				w.WriteHeader(400)
				w.Write([]byte(`{"ok":false,"error":"pedido inválido"}`))
				return
			}
			w.WriteHeader(429)
			w.Write([]byte(`{"ok":false,"error":"mandaste muchos mensajes seguidos: esperá un momento"}`))
		}
	}))
	defer srv.Close()
	viejo := urlVoz
	urlVoz = srv.URL + "/"
	defer func() { urlVoz = viejo }()

	a := &App{}
	if _, err := a.MensajesConversaciones(); err == nil || !strings.Contains(err.Error(), "entrá al chat") {
		t.Fatalf("sin chat tenía que pedir entrar: %v", err)
	}
	a.sesion.usuario, a.sesion.clave = "arena", "secreta"
	if r, err := a.MensajesConversaciones(); err != nil || !strings.Contains(r, `"sinLeer":1`) {
		t.Fatalf("conversaciones: %q %v", r, err)
	}
	a.MensajesConversaciones()
	if entradas != 1 {
		t.Fatalf("tenía que reusar el token: %d entradas", entradas)
	}
	token = "" // vence: se vuelve a entrar solo y se repite el pedido
	if _, err := a.MensajesConversaciones(); err != nil || entradas != 2 {
		t.Fatalf("no volvió a entrar al vencer: %v (%d)", err, entradas)
	}
	if _, err := a.MensajesEnviar("*Agos", "hola", `{"n":"Shako","c":"#c7b377","img":"uap"}`); err == nil || !strings.Contains(err.Error(), "muchos mensajes") {
		t.Fatalf("el error del servidor tenía que llegar tal cual: %v", err)
	}
	a.sesion.clave = "otra"
	a.olvidarMensajes()
	if _, err := a.MensajesConversaciones(); err == nil || err.Error() != "cuenta o contraseña incorrectas" {
		t.Fatalf("clave mala: %v", err)
	}
}
