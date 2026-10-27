package main

// HWID: huella de la maquina para que un baneo no se evada cambiando la IP. Se arma con la(s) MAC de
// las placas de red + un identificador estable de la maquina (MachineGuid en Windows), todo hasheado.
// El launcher la consulta al servidor antes de dejar jugar; si esta baneada, muestra la pantalla BANEADO.
//
// Aviso honesto: la MAC y el HWID tambien se pueden cambiar; esto frena al que solo cambia la IP. El
// candado fuerte es la validacion del mismo HWID en el servidor del juego (pendiente, ver AGENDA).

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// BanInfo lo lee el frontend para decidir si muestra la pantalla BANEADO.
type BanInfo struct {
	Baneado bool   `json:"baneado"`
	Motivo  string `json:"motivo"`
	Hwid    string `json:"hwid"`
}

// calcularHWID: MACs (ordenadas) + identificador de la maquina -> sha256 en hex.
func calcularHWID() string {
	var macs []string
	if ifaces, err := net.Interfaces(); err == nil {
		for _, it := range ifaces {
			if it.Flags&net.FlagLoopback != 0 {
				continue
			}
			m := it.HardwareAddr.String()
			if m != "" && m != "00:00:00:00:00:00" {
				macs = append(macs, m)
			}
		}
	}
	sort.Strings(macs)
	h := sha256.Sum256([]byte(strings.Join(macs, ",") + "|" + idMaquina()))
	return hex.EncodeToString(h[:])
}

// baseServidor: la raiz del servidor web (https://.../), derivada de URLDescargas.
func baseServidor() string {
	return strings.TrimSuffix(URLDescargas, "descargas/")
}

// EstadoBaneo (expuesto al frontend): calcula el HWID y le pregunta al servidor si esta baneado.
// Si el servidor no responde, NO banea (fail-open): el candado real es del lado del servidor del juego.
// El HWID se calcula una sola vez (no cambia mientras corre el launcher).
var (
	hwidUnaVez sync.Once
	hwidValor  string
	banMu      sync.Mutex
	banUltimo  *BanInfo // ultimo resultado del servidor (lo usa Jugar sin esperar a la red)
)

func hwidMaquina() string {
	hwidUnaVez.Do(func() { hwidValor = calcularHWID() })
	return hwidValor
}

// banGuardado: el ultimo estado de baneo conocido (nil si todavia no hubo respuesta del servidor).
func banGuardado() *BanInfo {
	banMu.Lock()
	defer banMu.Unlock()
	return banUltimo
}

func (a *App) EstadoBaneo() BanInfo {
	hwid := hwidMaquina()
	info := BanInfo{Hwid: hwid}

	cuerpo, _ := json.Marshal(map[string]string{"hwid": hwid})
	req, err := http.NewRequest(http.MethodPost, baseServidor()+"panel-api/hwid/check", bytes.NewReader(cuerpo))
	if err != nil {
		return info
	}
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 8 * time.Second}
	r, err := cli.Do(req)
	if err != nil {
		return info
	}
	defer r.Body.Close()
	var resp struct {
		OK     bool   `json:"ok"`
		Banned bool   `json:"banned"`
		Reason string `json:"reason"`
	}
	if json.NewDecoder(r.Body).Decode(&resp) == nil && resp.OK {
		info.Baneado = resp.Banned
		info.Motivo = resp.Reason
		banMu.Lock()
		copia := info
		banUltimo = &copia
		banMu.Unlock()
	}
	return info
}
