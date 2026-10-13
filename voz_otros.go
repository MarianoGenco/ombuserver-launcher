//go:build !windows

package main

// Fuera de Windows no hay tecla para hablar global (solo para probar la ventana).
func teclaApretada(int) bool { return false }
