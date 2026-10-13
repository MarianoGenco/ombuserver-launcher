//go:build !windows

package main

// Fuera de Windows (solo para compilar y probar) no se recuerda nada.

func guardarCredenciales(cuenta, clave string)          {}
func guardarSoloCuenta(cuenta string)                   {}
func borrarCredenciales()                               {}
func leerCredenciales() (cuenta, clave string, ok bool) { return "", "", false }
func ultimaCuentaBnet() string                          { return "" }
