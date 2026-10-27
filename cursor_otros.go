//go:build !windows

package main

// ponerCursorVentana: fuera de Windows no hace falta (el puntero lo pone la página, cursor.js).
func ponerCursorVentana(c *cursorMano) {}
