package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ombuserver-launcher/chat"
)

// Voz del chat: salas de voz de LiveKit, una por canal privado (las maneja la ventana con livekit-client). Acá solo
// lo que la ventana no puede hacer sola:
//   - pedir el permiso de entrar (y el de silenciar para todos) con la cuenta del chat, sin pasarle la contraseña a la
//     ventana: se manda el hash como lo guarda PvPGN y la API del VPS del juego lo compara con la base;
//   - la tecla para hablar, que tiene que andar aunque el juego tenga el foco: se lee el estado de la tecla cada
//     15 ms (GetAsyncKeyState; no engancha el teclado ni le saca la tecla al juego).
// Reglas (las aplica el servidor, voz-token.py): solo canales privados, solo quien está en el canal, y silenciar para
// todos solo el operador del canal (el que lo creó) o un admin.

// urlVoz: acá va la dirección (https) de tu servidor web, que pasa el permiso de voz y la señal de LiveKit
// (se pone al compilar con -ldflags -X; ver ombu/ombu.go).
var urlVoz = "https://IP-DEL-SERVIDOR-WEB/"

// teclaVozPorDefecto: botón lateral 4 del mouse (VK_XBUTTON1): no lo usa el juego.
const teclaVozPorDefecto = 0x05

// PermisoVoz es lo que necesita la ventana para entrar a la sala de voz.
type PermisoVoz struct {
	Token      string `json:"token"`
	URL        string `json:"url"`
	Identidad  string `json:"identidad"`
	Operador   bool   `json:"operador"`   // puede silenciar para todos
	Silenciado bool   `json:"silenciado"` // el operador le sacó la voz en esta sala
}

func (a *App) credencialesChat() (string, string, error) {
	s := &a.sesion
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.usuario == "" || s.clave == "" {
		return "", "", errors.New("entrá al chat para usar la voz")
	}
	return s.usuario, s.clave, nil
}

func pedirVoz(ruta string, pedido any, respuesta any) error {
	b, _ := json.Marshal(pedido)
	cli := &http.Client{Timeout: 20 * time.Second}
	r, err := cli.Post(urlVoz+ruta, "application/json", bytes.NewReader(b))
	if err != nil {
		return errors.New("no se pudo conectar con el servidor de voz")
	}
	defer r.Body.Close()
	if r.StatusCode == http.StatusTooManyRequests {
		return errors.New("demasiados intentos: esperá un minuto")
	}
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("el servidor de voz respondió %d", r.StatusCode)
	}
	var base struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
	}
	cuerpo := new(bytes.Buffer)
	cuerpo.ReadFrom(io.LimitReader(r.Body, 64<<10))
	if json.Unmarshal(cuerpo.Bytes(), &base) != nil {
		return errors.New("respuesta inválida del servidor de voz")
	}
	if !base.Ok {
		if base.Error == "" {
			base.Error = "el servidor de voz no dio permiso"
		}
		return errors.New(base.Error)
	}
	return json.Unmarshal(cuerpo.Bytes(), respuesta)
}

// VozPermiso pide el permiso para la sala de voz del canal.
func (a *App) VozPermiso(canal string) (PermisoVoz, error) {
	var p PermisoVoz
	cuenta, clave, err := a.credencialesChat()
	if err != nil {
		return p, err
	}
	err = pedirVoz("voz-token", map[string]string{
		"cuenta": cuenta, "hash": chat.HashClaveHex(clave), "sala": strings.TrimSpace(canal),
	}, &p)
	return p, err
}

// VozSilenciar le saca (o le devuelve) la voz a alguien para todos en la sala del canal. Solo el operador del canal.
func (a *App) VozSilenciar(canal, objetivo string, silenciar bool) error {
	cuenta, clave, err := a.credencialesChat()
	if err != nil {
		return err
	}
	var r struct{}
	return pedirVoz("voz-silenciar", map[string]any{
		"cuenta": cuenta, "hash": chat.HashClaveHex(clave), "sala": strings.TrimSpace(canal),
		"objetivo": strings.TrimPrefix(strings.TrimSpace(objetivo), "*"), "silenciar": silenciar,
	}, &r)
}

// GuardarVoz guarda las opciones de voz (sección Voz de la configuración).
func (a *App) GuardarVoz(modo, tecla int, nombreTecla string, sensibilidad int) (Config, error) {
	a.mu.Lock()
	c := a.cfg
	c.VozModo, c.VozTecla, c.VozTeclaNombre, c.VozSensibilidad = modo, tecla, nombreTecla, sensibilidad
	c = normalizar(c)
	if err := guardarConfig(c); err != nil {
		a.mu.Unlock()
		return a.cfg, err
	}
	a.cfg = c
	a.mu.Unlock()
	a.ptt.reiniciar(a)
	return c, nil
}

// teclaHablar mira la tecla para hablar mientras la ventana esté en una sala de voz con ese modo.
type teclaHablar struct {
	mu     sync.Mutex
	activa bool
	parar  chan struct{}
}

// VozTeclaHablar: la ventana avisa si hay que mirar la tecla (está en la voz con "tecla para hablar"). Cada vez que
// se aprieta o se suelta, emite "voz-tecla" con true o false.
func (a *App) VozTeclaHablar(activa bool) {
	a.ptt.mu.Lock()
	a.ptt.activa = activa
	a.ptt.mu.Unlock()
	a.ptt.reiniciar(a)
}

func (t *teclaHablar) reiniciar(a *App) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.parar != nil {
		close(t.parar)
		t.parar = nil
	}
	a.mu.Lock()
	vk, modo := a.cfg.VozTecla, a.cfg.VozModo
	a.mu.Unlock()
	if !t.activa || modo != 0 || a.ctx == nil {
		return
	}
	parar := make(chan struct{})
	t.parar = parar
	go func() {
		tic := time.NewTicker(15 * time.Millisecond)
		defer tic.Stop()
		apretada := false
		for {
			select {
			case <-parar:
				if apretada {
					runtime.EventsEmit(a.ctx, "voz-tecla", false)
				}
				return
			case <-tic.C:
				if ap := teclaApretada(vk); ap != apretada {
					apretada = ap
					runtime.EventsEmit(a.ctx, "voz-tecla", ap)
				}
			}
		}
	}()
}
