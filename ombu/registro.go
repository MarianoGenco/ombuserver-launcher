package ombu

import (
	"fmt"
	"strconv"
	"strings"
)

// Claves de registro (todas en HKCU: no hace falta ser administrador).
// Launcher/TODO_SETUP_PC_JUGADOR.md, ítems 1 y 2.
const (
	ClaveBattleNet  = `Software\Battle.net\Configuration`
	ValorGateways   = "Diablo II Battle.net Gateways"
	ClaveDiablo     = `Software\Blizzard Entertainment\Diablo II`
	ClaveCompat     = `Software\Microsoft\Windows NT\CurrentVersion\AppCompatFlags\Layers`
	CompatGameExe   = "~ WIN8RTM" // sin esto Fog.dll desborda la pila en Windows 11
	versionGateways = "1002"
)

// MezclarGateways agrega (o actualiza) el gateway de OmbuServer en la lista de Battle.net sin
// borrar los que el jugador ya tenga, y lo deja seleccionado.
// Formato del REG_MULTI_SZ: "1002", índice elegido (2 dígitos, desde 1), y de a tres:
// IP, zona horaria, nombre.
func MezclarGateways(actual []string, ip, zona, nombre string) []string {
	var trios [][3]string
	if len(actual) >= 2 && actual[0] == versionGateways {
		for i := 2; i+2 < len(actual); i += 3 {
			t := [3]string{actual[i], actual[i+1], actual[i+2]}
			if strings.EqualFold(t[2], nombre) || t[0] == ip {
				continue
			}
			trios = append(trios, t)
		}
	}
	trios = append(trios, [3]string{ip, zona, nombre})
	out := []string{versionGateways, fmt.Sprintf("%02d", len(trios))}
	for _, t := range trios {
		out = append(out, t[0], t[1], t[2])
	}
	return out
}

// IndiceElegido devuelve el gateway seleccionado de la lista (para mostrarlo/probarlo).
func IndiceElegido(lista []string) (string, bool) {
	if len(lista) < 2 {
		return "", false
	}
	n, err := strconv.Atoi(lista[1])
	i := 2 + 3*(n-1)
	if err != nil || n < 1 || i+2 >= len(lista) {
		return "", false
	}
	return lista[i+2], true
}
