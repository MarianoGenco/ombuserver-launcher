package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"ombuserver-launcher/ombu"
)

// Modo de prueba de carga (oculto): si junto a OmbuServer.exe hay un prueba-carga.txt, JUGAR abre una ventana por cada
// cuenta de la lista y en cada una escribe la cuenta y la contraseña en la pantalla de Battle.net y entra con el
// primer personaje (autologueo_windows.go). Las partidas se arman a mano. Sin el archivo, todo funciona como siempre.
//
//	# cuenta clave
//	carga01 carga2026
//	carga02 carga2026
//	espera_login 15        (opcional: segundos desde que aparece la ventana hasta escribir la cuenta)
//	espera_personaje 8     (opcional: segundos desde el login hasta entrar con el personaje)

const archivoPruebaCarga = "prueba-carga.txt"

// anotarPrueba agrega una línea a prueba-carga.log (junto al .txt), para ver qué hizo el autologueo en cada ventana.
func anotarPrueba(formato string, args ...any) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(filepath.Dir(exe), "prueba-carga.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s  %s\r\n", time.Now().Format("15:04:05"), fmt.Sprintf(formato, args...))
}

// registrarFallasPrueba: en modo prueba, si el launcher se cae, Go deja el motivo en prueba-carga-error.log.
func registrarFallasPrueba() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	if _, err := os.Stat(filepath.Join(dir, archivoPruebaCarga)); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "prueba-carga-error.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	fmt.Fprintf(f, "\r\n=== launcher %s abierto a las %s\r\n", Version, time.Now().Format("15:04:05"))
	debug.SetCrashOutput(f, debug.CrashOptions{})
}

type cuentaPrueba struct{ cuenta, clave string }

type pruebaCarga struct {
	cuentas         []cuentaPrueba
	esperaLogin     time.Duration
	esperaPersonaje time.Duration
}

// leerPruebaCarga devuelve nil si no hay archivo (lo normal).
func leerPruebaCarga() *pruebaCarga {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	f, err := os.Open(filepath.Join(filepath.Dir(exe), archivoPruebaCarga))
	if err != nil {
		return nil
	}
	defer f.Close()
	p := &pruebaCarga{esperaLogin: 15 * time.Second, esperaPersonaje: 8 * time.Second}
	s := bufio.NewScanner(f)
	for s.Scan() {
		campos := strings.Fields(s.Text())
		if len(campos) < 2 || strings.HasPrefix(campos[0], "#") {
			continue
		}
		if n, err := strconv.Atoi(campos[1]); err == nil && strings.HasPrefix(campos[0], "espera_") {
			if campos[0] == "espera_login" {
				p.esperaLogin = time.Duration(n) * time.Second
			} else if campos[0] == "espera_personaje" {
				p.esperaPersonaje = time.Duration(n) * time.Second
			}
			continue
		}
		p.cuentas = append(p.cuentas, cuentaPrueba{campos[0], campos[1]})
	}
	if len(p.cuentas) == 0 {
		anotarPrueba("%s no tiene cuentas: JUGAR funciona como siempre", archivoPruebaCarga)
		return nil
	}
	if len(p.cuentas) > 64 {
		p.cuentas = p.cuentas[:64]
	}
	anotarPrueba("modo prueba: %d cuentas, espera_login %v, espera_personaje %v", len(p.cuentas), p.esperaLogin, p.esperaPersonaje)
	return p
}

// juegosPrueba: los Game.exe que abrió la prueba de carga (se cierran al cerrar el launcher).
type juegosPrueba struct {
	mu       sync.Mutex
	procesos []*os.Process
}

// jugarPrueba abre una ventana por cuenta, de a una: graba su cuenta como recordada, abre el juego, espera la
// pantalla de login y escribe la contraseña (autologueo_windows.go); al final deja la cuenta recordada que había.
func (a *App) jugarPrueba(cfg Config, banderas []string, p *pruebaCarga) error {
	anterior, habia := cuentaRecordada()
	defer recordarCuenta(anterior, habia)
	for i, c := range p.cuentas {
		a.avisar(fmt.Sprintf("Prueba de carga: %s (%d de %d)…", c.cuenta, i+1, len(p.cuentas)), -1)
		recordarCuenta(c.cuenta, true)
		cmd, err := ombu.Lanzar(cfg.Carpeta, banderas)
		if err != nil {
			return fmt.Errorf("no se pudo abrir Game.exe: %v", err)
		}
		a.vigilarJuego(cmd)
		a.prueba.mu.Lock()
		a.prueba.procesos = append(a.prueba.procesos, cmd.Process)
		a.prueba.mu.Unlock()
		if h := loguearVentana(cmd.Process.Pid, c, p); h != 0 {
			go entrarPersonaje(h, c, p.esperaPersonaje)
		}
	}
	anotarPrueba("listo: %d ventanas", len(p.cuentas))
	return nil
}

// cerrarPrueba: al cerrar el launcher, cierra los juegos que abrió la prueba de carga.
func (a *App) cerrarPrueba(context.Context) {
	a.prueba.mu.Lock()
	defer a.prueba.mu.Unlock()
	for _, pr := range a.prueba.procesos {
		pr.Kill()
	}
	if len(a.prueba.procesos) > 0 {
		anotarPrueba("launcher cerrado: se cerraron %d juegos de la prueba", len(a.prueba.procesos))
	}
	a.prueba.procesos = nil
}
