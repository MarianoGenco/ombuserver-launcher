package main

import (
	"errors"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ombuserver-launcher/chat"
	"ombuserver-launcher/ombu"
)

// Chat de Battle.net en el launcher (pestaña CHAT). Los jugadores suelen chatear con una cuenta aparte, así que la
// sesión sigue abierta mientras juegan; si la cuenta del chat es la misma con la que se juega (la última que usó
// el juego, "Last BNet" en el registro), al tocar JUGAR se cierra y se vuelve a abrir cuando terminan los Game.exe
// que abrió el launcher (Battle.net no deja la misma cuenta conectada dos veces).

// direccionChat: Battle.net del servidor. Se arma al conectar (IPRealm se pone al compilar; ver ombu/ombu.go).
func direccionChat() string { return ombu.IPRealm + ":6112" }

type sesionChat struct {
	mu             sync.Mutex
	cli            *chat.Cliente
	usuario, clave string // en memoria, para reconectar (después del juego o si se corta)
	conectando     bool
	pausado        bool // cerrada a propósito mientras se juega con la misma cuenta
	juegos         int  // Game.exe abiertos por el launcher que siguen corriendo
	generacion     int  // cambia en cada inicio o cierre de sesión (descarta reconexiones viejas)
}

// EstadoChat es lo que la pestaña CHAT necesita para dibujarse.
type EstadoChat struct {
	Conectado  bool   `json:"conectado"`
	Conectando bool   `json:"conectando"`
	Pausado    bool   `json:"pausado"`
	Usuario    string `json:"usuario"`
	Recordado  string `json:"recordado"` // cuenta recordada (completa el login)
	Mensaje    string `json:"mensaje"`
}

func (a *App) estadoChat(mensaje string) EstadoChat {
	s := &a.sesion
	s.mu.Lock()
	e := EstadoChat{Conectado: s.cli != nil, Conectando: s.conectando, Pausado: s.pausado, Usuario: s.usuario, Mensaje: mensaje}
	s.mu.Unlock()
	if cuenta, _, ok := leerCredenciales(); ok {
		e.Recordado = cuenta
	}
	return e
}

func (a *App) avisarChat(mensaje string) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "chat-estado", a.estadoChat(mensaje))
	}
}

// ChatEstado: cómo está la sesión ahora.
func (a *App) ChatEstado() EstadoChat { return a.estadoChat("") }

// conectarChat abre la sesión (sin tocar lo recordado). gen descarta el intento si mientras tanto cambió la sesión.
func (a *App) conectarChat(usuario, clave string, gen int) error {
	s := &a.sesion
	cli, err := chat.Conectar(direccionChat(), usuario, clave,
		func(ev chat.Evento) { runtime.EventsEmit(a.ctx, "chat", ev) },
		func(error) { a.chatCortado(gen) })
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conectando = false
	if err != nil {
		return err
	}
	if gen != s.generacion {
		cli.Cerrar()
		return errors.New("sesión cancelada")
	}
	s.cli, s.usuario, s.clave, s.pausado = cli, usuario, clave, false
	return nil
}

// chatCortado: se cerró la conexión. Si no fue a propósito, se reintenta cada 10 segundos.
func (a *App) chatCortado(gen int) {
	s := &a.sesion
	s.mu.Lock()
	if gen != s.generacion || s.pausado {
		s.mu.Unlock()
		return
	}
	s.cli = nil
	usuario, clave := s.usuario, s.clave
	s.mu.Unlock()
	a.avisarChat("Se cortó la conexión con Battle.net. Reconectando…")
	go func() {
		for intento := 0; intento < 30; intento++ {
			time.Sleep(10 * time.Second)
			s.mu.Lock()
			if gen != s.generacion || s.pausado || s.cli != nil {
				s.mu.Unlock()
				return
			}
			s.conectando = true
			s.mu.Unlock()
			if err := a.conectarChat(usuario, clave, gen); err == nil {
				a.avisarChat("")
				return
			} else if errors.Is(err, chat.ErrLogin) {
				a.avisarChat(err.Error())
				return
			}
		}
	}()
}

// ChatEntrar inicia sesión. Con recordar, la cuenta y la contraseña quedan guardadas (cifradas con DPAPI) y el
// launcher entra solo la próxima vez.
func (a *App) ChatEntrar(usuario, clave string, recordar bool) (EstadoChat, error) {
	usuario = strings.TrimSpace(usuario)
	if usuario == "" || clave == "" {
		return a.estadoChat(""), errors.New("completá la cuenta y la contraseña")
	}
	s := &a.sesion
	s.mu.Lock()
	if s.cli != nil {
		s.cli.Cerrar()
		s.cli = nil
	}
	s.generacion++
	gen := s.generacion
	s.conectando, s.pausado = true, false
	s.mu.Unlock()
	a.avisarChat("")
	if err := a.conectarChat(usuario, clave, gen); err != nil {
		a.avisarChat("")
		return a.estadoChat(""), err
	}
	if recordar {
		guardarCredenciales(usuario, clave)
	} else {
		borrarCredenciales()
	}
	a.avisarChat("")
	return a.estadoChat(""), nil
}

// ChatSalir cierra la sesión y olvida la contraseña recordada (la cuenta queda para completar el login).
func (a *App) ChatSalir() {
	s := &a.sesion
	s.mu.Lock()
	s.generacion++
	if s.cli != nil {
		s.cli.Cerrar()
	}
	s.cli, s.usuario, s.clave, s.pausado, s.conectando = nil, "", "", false, false
	s.mu.Unlock()
	if cuenta, _, ok := leerCredenciales(); ok {
		guardarSoloCuenta(cuenta)
	}
	a.avisarChat("")
}

// ChatEnviar manda un mensaje o un comando del chat ("/w *cuenta hola", "/join canal", "/me …").
func (a *App) ChatEnviar(texto string) error {
	s := &a.sesion
	s.mu.Lock()
	cli := s.cli
	s.mu.Unlock()
	if cli == nil {
		return errors.New("no estás conectado al chat")
	}
	return cli.Enviar(texto)
}

// ChatCanal cambia de canal.
func (a *App) ChatCanal(nombre string) error {
	s := &a.sesion
	s.mu.Lock()
	cli := s.cli
	s.mu.Unlock()
	if cli == nil {
		return errors.New("no estás conectado al chat")
	}
	return cli.UnirCanal(strings.TrimSpace(nombre))
}

// chatAlArrancar entra solo con la cuenta recordada.
func (a *App) chatAlArrancar() {
	cuenta, clave, ok := leerCredenciales()
	if !ok || clave == "" {
		return
	}
	go func() {
		if _, err := a.ChatEntrar(cuenta, clave, true); err != nil {
			a.avisarChat(err.Error())
		}
	}()
}

// juegoAbierto: antes de abrir un Game.exe. Si el chat usa la misma cuenta que el juego, se cierra hasta que
// terminen los juegos que abrió el launcher.
func (a *App) antesDeJugar() {
	s := &a.sesion
	s.mu.Lock()
	misma := s.cli != nil && s.usuario != "" && strings.EqualFold(s.usuario, ultimaCuentaBnet())
	if misma {
		s.pausado = true
		s.cli.Cerrar()
		s.cli = nil
	}
	s.mu.Unlock()
	if misma {
		a.avisarChat("Chat en pausa mientras jugás con esta cuenta.")
	}
}

// vigilarJuego cuenta los Game.exe abiertos y, cuando terminan todos, vuelve a abrir el chat si estaba en pausa.
func (a *App) vigilarJuego(cmd *exec.Cmd) {
	s := &a.sesion
	s.mu.Lock()
	s.juegos++
	s.mu.Unlock()
	go func() {
		cmd.Wait()
		s.mu.Lock()
		s.juegos--
		volver := s.juegos <= 0 && s.pausado
		usuario, clave, gen := s.usuario, s.clave, s.generacion
		if volver {
			s.conectando = true
		}
		s.mu.Unlock()
		if !volver {
			return
		}
		time.Sleep(3 * time.Second) // que Battle.net termine de cerrar la sesión del juego
		if err := a.conectarChat(usuario, clave, gen); err != nil {
			s.mu.Lock()
			s.pausado = false
			s.mu.Unlock()
			a.avisarChat("No se pudo volver a entrar al chat: " + err.Error())
			return
		}
		a.avisarChat("")
	}()
}
