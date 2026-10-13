//go:build !windows

package main

// Fuera de Windows no hay autologueo (el juego es de Windows).
func autologuear(int, cuentaPrueba, *pruebaCarga) {}
