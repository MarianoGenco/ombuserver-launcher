//go:build !windows

package main

import "time"

// Fuera de Windows no hay autologueo (el juego es de Windows).
func cuentaRecordada() (string, bool)                      { return "", false }
func recordarCuenta(string, bool)                          {}
func esperarVentana(int, cuentaPrueba) uintptr             { return 0 }
func escribirClave(uintptr, cuentaPrueba) bool             { return false }
func entrarPersonaje(uintptr, cuentaPrueba, time.Duration) {}
