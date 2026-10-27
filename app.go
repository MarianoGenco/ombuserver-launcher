package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ombuserver-launcher/chat"
	"ombuserver-launcher/ombu"
)

// App es lo que la ventana ve (window.go.main.App.*). Cada método corre fuera del hilo de la
// interfaz, así que puede bajar archivos sin trabar la ventana.
type App struct {
	ctx      context.Context
	mu       sync.Mutex
	cfg      Config
	ocupado  bool
	cliente  *ombu.Cliente
	juego    *ombu.Manifiesto // archivos del juego publicados (se baja una vez por sesión)
	sesion   sesionChat       // chat de Battle.net (chat_app.go)
	ptt      teclaHablar      // tecla para hablar en la voz (voz.go)
	mensajes sesionMensajes   // mensajes entre jugadores por los TRADES (mensajes.go)

	// juegos abiertos con JUGAR: se cierran al cerrar el launcher (salvo cuando se cierra para actualizarse)
	juegosMu       sync.Mutex
	juegos         []*os.Process
	actualizandose bool
}

// cerrarJuegos: al cerrar el launcher, cierra los Diablo II que abrió (pedido del usuario). Si se cierra para abrir
// la versión nueva, no los toca.
func (a *App) cerrarJuegos(context.Context) {
	a.juegosMu.Lock()
	defer a.juegosMu.Unlock()
	if a.actualizandose {
		return
	}
	for _, p := range a.juegos {
		p.Kill() // si ya se cerró, no hace nada
	}
	a.juegos = nil
}

func NuevaApp() *App {
	c := ombu.NuevoCliente()
	return &App{cfg: cargarConfig(), cliente: c}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg.Carpeta == "" {
		if d := detectarCarpeta(); d != "" {
			a.cfg.Carpeta = d
			guardarConfig(a.cfg)
		}
	}
	if a.cfg.Carpeta != "" {
		ombu.LimpiarCarpetaVieja(ombu.CarpetaDatos(a.cfg.Carpeta, ""), "")
	}
	a.chatAlArrancar()
}

func (a *App) otraInstancia(options.SecondInstanceData) {
	runtime.WindowUnminimise(a.ctx)
	runtime.Show(a.ctx)
}

// detectarCarpeta busca un Diablo II 1.14b en los lugares habituales: la carpeta del launcher (si
// lo pusieron junto al juego), la que anotó el instalador y las rutas de instalación de siempre.
func detectarCarpeta() string {
	var candidatas []string
	if exe, err := os.Executable(); err == nil {
		candidatas = append(candidatas, filepath.Dir(exe))
	}
	candidatas = append(candidatas, carpetaInstalada(),
		`C:\Program Files (x86)\Diablo II`, `C:\Program Files\Diablo II`, `C:\Diablo II`)
	for _, c := range candidatas {
		if c == "" {
			continue
		}
		if ok, _, _ := ombu.VerificarGameExe(c); ok {
			return c
		}
	}
	return ""
}

// Estado es lo que muestra la pantalla de inicio.
type Estado struct {
	Version   string `json:"version"`
	Config    Config `json:"config"`
	JuegoOK   bool   `json:"juegoOk"`   // la carpeta tiene el Game.exe 1.14b
	Pendiente bool   `json:"pendiente"` // hay algo para bajar
	Accion    string `json:"accion"`    // botón grande: "jugar", "actualizar", "instalar" o "" (deshabilitado)
	Launcher  string `json:"launcher"`  // versión nueva del launcher publicada ("" si no hay)
	Recien    bool   `json:"recien"`    // el launcher se acaba de actualizar a esta versión
	Aviso     string `json:"aviso"`     // "ok", "pendiente", "error" o "carpeta"
	Mensaje   string `json:"mensaje"`
}

// Estado revisa la carpeta del juego y compara con lo publicado en el servidor.
func (a *App) Estado() Estado {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()
	e := Estado{Version: Version, Config: cfg, Recien: recienActualizado}
	if v, err := a.versionPublicada(); err == nil && v != nil && masNueva(v.Version, Version) {
		e.Launcher, e.Pendiente, e.Accion = v.Version, true, "actualizar"
		e.Aviso, e.Mensaje = "pendiente", "Nueva versión del launcher"
		return e
	}
	if cfg.Carpeta == "" {
		e.Accion = "instalar"
		e.Aviso, e.Mensaje = "carpeta", "Elegí la carpeta de Diablo II"
		return e
	}
	mj, _ := a.manifiestoJuego()
	if faltan := faltantesJuego(cfg.Carpeta, mj); len(faltan) > 0 {
		e.Accion = "instalar"
		if pareceJuego(cfg.Carpeta, mj) {
			e.Aviso, e.Mensaje = "pendiente", "Faltan archivos del juego"
		} else {
			e.Aviso, e.Mensaje = "carpeta", "Elegí la carpeta de Diablo II"
		}
		return e
	}
	ok, sha, err := ombu.VerificarGameExe(cfg.Carpeta)
	switch {
	case err != nil:
		e.Aviso, e.Mensaje = "carpeta", "No se puede leer esa carpeta: "+err.Error()
		return e
	case sha == "":
		e.Accion = "instalar"
		e.Aviso, e.Mensaje = "carpeta", "Elegí la carpeta de Diablo II"
		return e
	case !ok:
		e.Aviso, e.Mensaje = "carpeta", "Esa carpeta tiene otra versión de Diablo II. OmbuServer necesita la 1.14b."
		return e
	}
	e.JuegoOK, e.Accion = true, "jugar"
	acciones, _, err := a.cliente.PendientesCliente(cfg.Carpeta)
	if err != nil {
		e.Aviso, e.Mensaje = "error", "No se pudo conectar con OmbuServer. Revisá tu conexión."
		return e
	}
	remota, err := a.cliente.VersionRemotaMPQ()
	if err != nil {
		e.Aviso, e.Mensaje = "error", "No se pudo conectar con OmbuServer. Revisá tu conexión."
		return e
	}
	local := ombu.VersionLocalMPQ(ombu.CarpetaDatos(cfg.Carpeta, ""))
	if len(acciones) > 0 || local != remota {
		e.Pendiente, e.Accion = true, "actualizar"
		e.Aviso, e.Mensaje = "pendiente", "Hay una actualización de OmbuServer."
		return e
	}
	e.Aviso, e.Mensaje = "ok", "El juego está al día."
	return e
}

// ChequeoRapido devuelve una "firma" de lo que falta en la carpeta (juego, DLL y MPQ), mirando solo si existen y su
// tamaño: la ventana la consulta cada pocos segundos y, si cambia, vuelve a revisar todo (Estado).
func (a *App) ChequeoRapido() string {
	a.mu.Lock()
	cfg, m := a.cfg, a.juego
	a.mu.Unlock()
	if cfg.Carpeta == "" {
		return "sin-carpeta"
	}
	faltan := faltantesJuego(cfg.Carpeta, m)
	for _, f := range []string{filepath.Join(cfg.Carpeta, "ijl11.dll"), filepath.Join(ombu.CarpetaDatos(cfg.Carpeta, ""), ombu.ArchivoMPQ)} {
		if st, err := os.Stat(f); err != nil {
			faltan = append(faltan, filepath.Base(f))
		} else {
			faltan = append(faltan, filepath.Base(f)+"="+strconv.FormatInt(st.Size(), 10))
		}
	}
	return strings.Join(faltan, "|")
}

// UsarIdioma: idioma del launcher ("es" o "en"), que elige la ventana (lo traduce ella; i18n.js). Acá cambia el
// diálogo de la carpeta y el idioma en que se presenta el chat, para que Battle.net conteste en ese idioma.
func (a *App) UsarIdioma(codigo string) {
	chat.Ingles.Store(codigo == "en")
}

// ElegirCarpeta abre el diálogo de Windows para elegir la carpeta del juego.
func (a *App) ElegirCarpeta() (Estado, error) {
	a.mu.Lock()
	actual := a.cfg.Carpeta
	a.mu.Unlock()
	titulo := "Elegí la carpeta de Diablo II (donde está Game.exe)"
	if chat.Ingles.Load() {
		titulo = "Choose the Diablo II folder (where Game.exe is)"
	}
	d, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            titulo,
		DefaultDirectory: actual,
	})
	if err != nil {
		return a.Estado(), err
	}
	if d != "" {
		a.mu.Lock()
		a.cfg.Carpeta = d
		err = guardarConfig(a.cfg)
		a.mu.Unlock()
	}
	return a.Estado(), err
}

// GuardarOpciones guarda lo que se cambió en la configuración.
func (a *App) GuardarOpciones(ventanas, demora int, ventana, sonido bool, idioma, escala, vista int) (Config, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	c := a.cfg
	c.Ventanas, c.Demora, c.Ventana, c.Sonido, c.Idioma, c.Escala, c.Vista = ventanas, demora, ventana, sonido, idioma, escala, vista
	c = normalizar(c)
	if err := guardarConfig(c); err != nil {
		return a.cfg, err
	}
	a.cfg = c
	return c, nil
}

// Progreso es lo que recibe la ventana en el evento "progreso".
type Progreso struct {
	Texto      string `json:"texto"`
	Porcentaje int    `json:"porcentaje"` // -1 = sin barra
}

func (a *App) avisar(texto string, porcentaje int) {
	runtime.EventsEmit(a.ctx, "progreso", Progreso{Texto: texto, Porcentaje: porcentaje})
}

// barra devuelve un ombu.Progreso que avisa a la ventana como mucho cada 100 ms.
func (a *App) barra(texto string) ombu.Progreso {
	var ultimo time.Time
	return func(hechos, total int64) {
		if total <= 0 || time.Since(ultimo) < 100*time.Millisecond && hechos < total {
			return
		}
		ultimo = time.Now()
		a.avisar(texto, int(hechos*100/total))
	}
}

// ocupar marca que hay una tarea en curso (actualizar o jugar); devuelve la función para liberarla.
func (a *App) ocupar() (Config, func(), error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ocupado {
		return Config{}, nil, errors.New("ya hay una tarea en curso")
	}
	a.ocupado = true
	return a.cfg, func() {
		a.mu.Lock()
		a.ocupado = false
		a.mu.Unlock()
	}, nil
}

// Actualizar deja todo al día sin abrir el juego. Si lo nuevo es el launcher, se reemplaza,
// abre la versión nueva y este se cierra (devuelve true).
func (a *App) Actualizar() (bool, error) {
	cfg, liberar, err := a.ocupar()
	if err != nil {
		return false, err
	}
	defer liberar()
	a.avisar("Buscando actualizaciones…", -1)
	v, err := a.versionPublicada()
	if err != nil {
		return false, sinConexion(err)
	}
	if v != nil && masNueva(v.Version, Version) {
		if err := a.actualizarLauncher(v); err != nil {
			return false, err
		}
		a.avisar("Abriendo la versión nueva…", -1)
		go func() {
			time.Sleep(400 * time.Millisecond)
			a.juegosMu.Lock()
			a.actualizandose = true // se cierra para abrir la versión nueva: los juegos quedan abiertos
			a.juegosMu.Unlock()
			runtime.Quit(a.ctx)
		}()
		return true, nil
	}
	return false, a.actualizarJuego(cfg)
}

// consultaReciente: cuanto vale lo ultimo que contesto el servidor para JUGAR sin volver a preguntar.
const consultaReciente = 10 * time.Minute

// actualizarJuego verifica el Game.exe y deja al día la DLL (carpeta del juego) y el contenido del
// Mundo 6 (carpeta de datos).
func (a *App) actualizarJuego(cfg Config) error {
	a.avisar("Revisando el juego…", -1)
	ok, sha, err := ombu.VerificarGameExe(cfg.Carpeta)
	if err != nil {
		return err
	}
	if !ok {
		if sha == "" {
			return errors.New("en la carpeta elegida no está Game.exe")
		}
		return errors.New("la carpeta elegida tiene otra versión de Diablo II: OmbuServer necesita la 1.14b")
	}
	// JUGAR usa lo que contesto el servidor en los ultimos minutos (el launcher lo consulta al abrirse y al volver a
	// la ventana): asi abre al instante en vez de esperar dos consultas por HTTPS. Los archivos locales se revisan igual.
	acciones, _, err := a.cliente.PendientesClienteReciente(cfg.Carpeta, consultaReciente)
	if err != nil {
		return sinConexion(err)
	}
	if len(acciones) > 0 {
		a.avisar("Actualizando OmbuServer…", 0)
		if err := a.cliente.AplicarCliente(cfg.Carpeta, acciones, a.barra("Actualizando OmbuServer…")); err != nil {
			return sinConexion(err)
		}
	}
	datos := ombu.CarpetaDatos(cfg.Carpeta, "")
	if _, err := a.cliente.SincronizarMPQReciente(datos, a.barra("Bajando actualización…"), consultaReciente); err != nil {
		return sinConexion(err)
	}
	return nil
}

// Jugar deja todo al día y abre el juego: DLL de la carpeta del juego, contenido del Mundo 6,
// registro de Windows (gateway de OmbuServer) y Game.exe (las ventanas que haya elegido).
func (a *App) Jugar() error {
	// Candado por HWID: si la maquina esta baneada, no se abre el juego (el frontend muestra BANEADO).
	// Se usa el ultimo estado conocido (lo consulta el frontend al abrir el launcher): Jugar no espera a la red
	// (antes la consulta sumaba ~2 s de "Preparando..."). Se refresca en segundo plano para la proxima vez.
	ban := BanInfo{Hwid: hwidMaquina()}
	if g := banGuardado(); g != nil {
		ban = *g
	}
	go a.EstadoBaneo()
	if ban.Baneado {
		if ban.Motivo != "" {
			return fmt.Errorf("BANEADO: %s", ban.Motivo)
		}
		return fmt.Errorf("BANEADO")
	}
	// Se lo pasamos al juego por el entorno: la DLL (mod_hwid.cpp) lo manda al D2GS asi coincide EXACTO
	// con lo que el panel banea. Candado fuerte del lado del servidor del juego (ombu_hwid.c).
	if ban.Hwid != "" {
		os.Setenv("OMBU_HWID", ban.Hwid)
	}
	cfg, liberar, err := a.ocupar()
	if err != nil {
		return err
	}
	defer liberar()
	if err := a.actualizarJuego(cfg); err != nil {
		return err
	}

	a.avisar("Configurando Battle.net…", -1)
	if err := ombu.ConfigurarRegistro(cfg.Carpeta); err != nil {
		return fmt.Errorf("no se pudo configurar Battle.net: %v", err)
	}

	banderas := []string{"-skiptobnet"}
	if cfg.Ventana {
		banderas = append([]string{"-w"}, banderas...)
	}
	if !cfg.Sonido {
		banderas = append(banderas, "-ns")
	}
	a.antesDeJugar()
	for i := 0; i < cfg.Ventanas; i++ {
		if cfg.Ventanas > 1 {
			a.avisar(fmt.Sprintf("Abriendo Diablo II (%d de %d)…", i+1, cfg.Ventanas), -1)
		} else {
			a.avisar("Abriendo Diablo II…", -1)
		}
		cmd, err := ombu.Lanzar(cfg.Carpeta, banderas)
		if err != nil {
			return fmt.Errorf("no se pudo abrir Game.exe: %v", err)
		}
		a.vigilarJuego(cmd)
		a.juegosMu.Lock()
		a.juegos = append(a.juegos, cmd.Process)
		a.juegosMu.Unlock()
		if i < cfg.Ventanas-1 {
			time.Sleep(time.Duration(cfg.Demora) * time.Second)
		}
	}
	return nil
}

// sinConexion traduce los errores de red a algo que el jugador entienda; el resto pasa igual.
func sinConexion(err error) error {
	if errors.Is(err, ombu.ErrEnUso) {
		return err
	}
	m := err.Error()
	for _, s := range []string{"dial tcp", "no such host", "timeout", "connection refused", "estado 5", "estado 404", "EOF"} {
		if strings.Contains(m, s) {
			return errors.New("no se pudo conectar con OmbuServer: revisá tu conexión y volvé a intentar")
		}
	}
	return err
}

// Noticia del panel derecho (noticias.json publicado en el servidor).
type Noticia struct {
	Titulo   string `json:"title"`
	Texto    string `json:"text"`
	TituloEn string `json:"title_en,omitempty"` // versión en inglés (la muestra app.js con el launcher en inglés)
	TextoEn  string `json:"text_en,omitempty"`
	Fecha    string `json:"date"`
	Enlace   string `json:"link"`
}

var noticiasPorDefecto = []Noticia{{
	Titulo: "Bienvenido a OmbuServer",
	Texto:  "Diablo II: Lord of Destruction 1.14b con el Mundo 6: Dún Fiacla, escenarios nuevos y misiones con voces. Tocá JUGAR y el launcher se encarga del resto.",
}}

// Noticias devuelve las novedades publicadas (como mucho 4); sin conexión, una bienvenida.
func (a *App) Noticias() []Noticia {
	t, err := a.cliente.Texto("noticias.json")
	if err != nil {
		return noticiasPorDefecto
	}
	var n []Noticia
	if json.Unmarshal([]byte(t), &n) != nil || len(n) == 0 {
		return noticiasPorDefecto
	}
	if len(n) > 4 {
		n = n[:4]
	}
	return n
}

// AbrirEnlace abre un enlace de una novedad en el navegador.
func (a *App) AbrirEnlace(url string) {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		runtime.BrowserOpenURL(a.ctx, url)
	}
}

// AbrirCarpeta abre en el Explorador la carpeta del juego ("juego") o la de datos ("datos").
func (a *App) AbrirCarpeta(cual string) {
	a.mu.Lock()
	d := a.cfg.Carpeta
	a.mu.Unlock()
	if cual == "datos" {
		d = ombu.CarpetaDatos(d, "")
	}
	if d != "" {
		exec.Command("explorer", d).Start()
	}
}

// Info es lo que muestra la sección "Acerca de".
type Info struct {
	Version      string `json:"version"`
	Servidor     string `json:"servidor"`
	CarpetaDatos string `json:"carpetaDatos"`
}

func (a *App) Info() Info {
	a.mu.Lock()
	d := a.cfg.Carpeta
	a.mu.Unlock()
	return Info{Version: Version, Servidor: ombu.NombreRealm, CarpetaDatos: ombu.CarpetaDatos(d, "")}
}
