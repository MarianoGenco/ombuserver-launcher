package ombu

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SHA1Archivo en hex.
func SHA1Archivo(ruta string) (string, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerificarGameExe dice si la carpeta tiene el Game.exe 1.14b para el que está hecha la DLL.
// Devuelve también el SHA1 encontrado ("" si no hay Game.exe) para mostrarlo en el error.
func VerificarGameExe(carpetaJuego string) (bool, string, error) {
	s, err := SHA1Archivo(filepath.Join(carpetaJuego, "Game.exe"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, "", nil
		}
		return false, "", err
	}
	return strings.EqualFold(s, SHA1GameExe114b), s, nil
}

// Lanzar arranca Game.exe (en 1.14b es el ejecutable del juego; "Diablo II.exe" es solo un
// lanzador que lo llama) en la carpeta del juego, con las banderas dadas.
func Lanzar(carpetaJuego string, banderas []string) (*exec.Cmd, error) {
	cmd := exec.Command(filepath.Join(carpetaJuego, "Game.exe"), banderas...)
	cmd.Dir = carpetaJuego
	// Marca para la DLL (OmbuServer-dll/src/mod_lanzador.cpp): el juego lo abrió el launcher.
	cmd.Env = append(os.Environ(), "OMBU_LAUNCHER="+strconv.FormatInt(time.Now().Unix(), 10))
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}
