//go:build !windows

package ombu

// ConfigurarRegistro no hace nada fuera de Windows (el juego solo corre en Windows; esto
// permite compilar y probar el núcleo en Linux).
func ConfigurarRegistro(carpetaJuego string) error { return nil }
