package main

// Mensajes entre jugadores por los TRADES (2026-10-14): desde TRADES se le escribe al dueño de un ítem y en CHAT,
// la columna de mensajes lista las conversaciones (chat privado que queda guardado aunque el otro no esté conectado).
// Los guarda el VPS web (<web>/trades-api/mensajes/, ver TRADES/web-servidor/mensajes.py).
//
// Se usan con la cuenta del chat: para entrar se manda la cuenta y el hash de la contraseña como lo guarda PvPGN
// (como la voz; la contraseña nunca sale de la PC) y el servidor, después de validarlos contra la base de cuentas
// del juego, devuelve un token de sesión. El token queda solo en memoria y va en cada pedido; la ventana no lo ve
// nunca. Si el servidor dice que venció, se vuelve a entrar solo una vez y se repite el pedido.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"ombuserver-launcher/chat"
)

type sesionMensajes struct {
	mu     sync.Mutex
	cuenta string // con qué cuenta del chat se sacó el token (si cambia la cuenta, se saca otro)
	token  string
}

var errSesionVencida = errors.New("sesión vencida")

func (a *App) tokenMensajes(forzar bool) (string, error) {
	cuenta, clave, err := a.credencialesChat()
	if err != nil {
		return "", errors.New("entrá al chat para usar los mensajes")
	}
	m := &a.mensajes
	m.mu.Lock()
	defer m.mu.Unlock()
	if !forzar && m.token != "" && strings.EqualFold(m.cuenta, cuenta) {
		return m.token, nil
	}
	var r struct {
		Token string `json:"token"`
	}
	if err := pedirMensajes(http.MethodPost, "entrar", "", map[string]string{
		"cuenta": cuenta, "hash": chat.HashClaveHex(clave)}, &r); err != nil {
		m.token = ""
		if errors.Is(err, errSesionVencida) {
			return "", errors.New("cuenta o contraseña incorrectas")
		}
		return "", err
	}
	m.cuenta, m.token = cuenta, r.Token
	return m.token, nil
}

// olvidarMensajes: al salir del chat (o cambiar de cuenta) el token viejo no se usa más.
func (a *App) olvidarMensajes() {
	a.mensajes.mu.Lock()
	a.mensajes.cuenta, a.mensajes.token = "", ""
	a.mensajes.mu.Unlock()
}

// pedirMensajes hace un pedido a la API de mensajes. Con token "" va sin autorización (solo para entrar).
func pedirMensajes(metodo, ruta, token string, cuerpo any, respuesta any) error {
	var datos io.Reader
	if cuerpo != nil {
		b, _ := json.Marshal(cuerpo)
		datos = bytes.NewReader(b)
	}
	req, err := http.NewRequest(metodo, urlVoz+"trades-api/mensajes/"+ruta, datos)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	cli := &http.Client{Timeout: 20 * time.Second}
	r, err := cli.Do(req)
	if err != nil {
		return errors.New("no se pudo conectar con OmbuServer")
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	var base struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if json.Unmarshal(b, &base) != nil {
		if r.StatusCode == http.StatusTooManyRequests {
			return errors.New("demasiados pedidos seguidos: esperá un momento")
		}
		return fmt.Errorf("el servidor respondió %d", r.StatusCode)
	}
	if r.StatusCode == http.StatusUnauthorized {
		return errSesionVencida
	}
	if !base.Ok {
		if base.Error == "" {
			base.Error = fmt.Sprintf("el servidor respondió %d", r.StatusCode)
		}
		return errors.New(base.Error)
	}
	if respuesta == nil {
		return nil
	}
	return json.Unmarshal(b, respuesta)
}

// conSesion hace el pedido con el token de la cuenta del chat; si venció, entra de nuevo y lo repite una vez.
// Devuelve el JSON tal cual (la ventana lo lee).
func (a *App) conSesion(metodo, ruta string, cuerpo any) (string, error) {
	for intento := 0; intento < 2; intento++ {
		token, err := a.tokenMensajes(intento > 0)
		if err != nil {
			return "", err
		}
		var crudo json.RawMessage
		err = pedirMensajes(metodo, ruta, token, cuerpo, &crudo)
		if errors.Is(err, errSesionVencida) {
			continue
		}
		return string(crudo), err
	}
	return "", errors.New("no se pudo entrar a los mensajes")
}

// MensajesConversaciones: las conversaciones de la cuenta del chat (columna de mensajes de CHAT).
func (a *App) MensajesConversaciones() (string, error) {
	return a.conSesion(http.MethodGet, "conversaciones", nil)
}

// MensajesConversacion: los mensajes con otra cuenta, desde el id que se pase (0 = los últimos 200). Los recibidos
// quedan leídos.
func (a *App) MensajesConversacion(con string, desde int) (string, error) {
	v := url.Values{}
	v.Set("con", strings.TrimPrefix(strings.TrimSpace(con), "*"))
	v.Set("desde", fmt.Sprint(desde))
	return a.conSesion(http.MethodGet, "conversacion?"+v.Encode(), nil)
}

// MensajesEnviar le manda un mensaje a otra cuenta. itemJSON es el ítem de TRADES del que se habla ({n, c, img}) o "".
func (a *App) MensajesEnviar(para, texto, itemJSON string) (string, error) {
	cuerpo := map[string]any{"para": strings.TrimPrefix(strings.TrimSpace(para), "*"), "texto": texto}
	if itemJSON != "" {
		var it map[string]any
		if json.Unmarshal([]byte(itemJSON), &it) == nil {
			cuerpo["item"] = it
		}
	}
	return a.conSesion(http.MethodPost, "enviar", cuerpo)
}

// MensajesBorrar borra para mí un mensaje (id > 0) o toda la conversación con una cuenta (id = 0). El otro conserva
// su copia.
func (a *App) MensajesBorrar(id int, con string) error {
	cuerpo := map[string]any{"con": strings.TrimPrefix(strings.TrimSpace(con), "*")}
	if id > 0 {
		cuerpo = map[string]any{"id": id}
	}
	_, err := a.conSesion(http.MethodPost, "borrar", cuerpo)
	return err
}
