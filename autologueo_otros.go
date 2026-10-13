//go:build !windows

package main

import "time"

// Fuera de Windows no hay autologueo (el juego es de Windows).
func cuentaRecordada() (string, bool)                        { return "", false }
func recordarCuenta(string, bool)                            {}
func loguearVentana(int, cuentaPrueba, *pruebaCarga) uintptr { return 0 }
func entrarPersonaje(uintptr, cuentaPrueba, time.Duration)   {}
