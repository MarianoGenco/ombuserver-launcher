package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ombuserver-launcher/ombu"
)

// Config es lo que el jugador elige en el launcher. Se guarda en el registro
// (HKCU\Software\OmbuServer\Launcher); fuera de Windows, en un config.json (solo para pruebas).
type Config struct {
	Carpeta  string `json:"carpeta"`  // carpeta del juego (donde está Game.exe)
	Ventanas int    `json:"ventanas"` // cuántos Diablo abrir con JUGAR
	Demora   int    `json:"demora"`   // segundos entre una ventana y la siguiente
	Ventana  bool   `json:"ventana"`  // modo ventana (-w): el banner del lobby lo necesita
	Sonido   bool   `json:"sonido"`   // false = -ns (sin sonido)
	Idioma   int    `json:"idioma"`   // idioma del juego: 0 inglés, 1 español (lo aplica la DLL, mod_mpq.cpp)
	Escala   int    `json:"escala"`   // tamaño de ventana: 100 (800×600, vista normal) o 150 (1200×900, vista alejada)
	Vista    int    `json:"vista"`    // 0 normal, 1 alejada (la partida a 1200×900, se ve más mapa); mod_vista.cpp

	// Voz del chat (voz.go): cómo se habla en la sala de voz de un canal privado.
	VozModo         int    `json:"vozModo"`         // 0 tecla para hablar, 1 detección de voz
	VozTecla        int    `json:"vozTecla"`        // tecla virtual de Windows (VK_*); también botones del mouse
	VozTeclaNombre  string `json:"vozTeclaNombre"`  // cómo se muestra la tecla
	VozSensibilidad int    `json:"vozSensibilidad"` // detección de voz: 0 (hay que hablar fuerte) a 100 (capta todo)
}

func configPorDefecto() Config {
	return Config{Ventanas: 1, Demora: 3, Ventana: true, Sonido: true, Idioma: 1, Escala: 100,
		VozTecla: teclaVozPorDefecto, VozTeclaNombre: "Mouse 4", VozSensibilidad: 50}
}

// rutaConfigVieja: donde la guardaban las versiones anteriores (se migra al registro y se borra).
func rutaConfigVieja() string {
	return filepath.Join(ombu.CarpetaVieja(""), "launcher", "config.json")
}

func cargarConfig() Config {
	c := configPorDefecto()
	if leerConfigGuardada(&c) {
		return normalizar(c)
	}
	if b, err := os.ReadFile(rutaConfigVieja()); err == nil && json.Unmarshal(b, &c) == nil {
		if guardarConfig(normalizar(c)) == nil {
			os.RemoveAll(filepath.Dir(rutaConfigVieja()))
		}
	}
	return normalizar(c)
}

func normalizar(c Config) Config {
	if c.Ventanas < 1 {
		c.Ventanas = 1
	}
	if c.Ventanas > 8 {
		c.Ventanas = 8
	}
	if c.Demora < 0 {
		c.Demora = 0
	}
	if c.Demora > 30 {
		c.Demora = 30
	}
	if c.Idioma != 0 {
		c.Idioma = 1
	}
	if c.Escala != 150 {
		c.Escala = 100
	}
	// 1200×900 es la vista alejada y 800×600 la normal (una sola opción en el launcher, 2026-10-14)
	if c.Escala == 150 {
		c.Vista = 1
	} else {
		c.Vista = 0
	}
	if c.VozModo != 1 {
		c.VozModo = 0
	}
	if c.VozTecla <= 0 || c.VozTecla > 0xFE || c.VozTecla == 0x01 || c.VozTecla == 0x02 {
		c.VozTecla, c.VozTeclaNombre = teclaVozPorDefecto, "Mouse 4"
	}
	if c.VozSensibilidad < 0 {
		c.VozSensibilidad = 0
	}
	if c.VozSensibilidad > 100 {
		c.VozSensibilidad = 100
	}
	return c
}

func guardarConfig(c Config) error { return escribirConfigGuardada(c) }
