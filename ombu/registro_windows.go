//go:build windows

package ombu

import (
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// ConfigurarRegistro deja la PC lista para OmbuServer, todo en HKCU (sin administrador):
//   - el gateway de Battle.net (sin borrar los que ya tenga el jugador) y BNETIP;
//   - el modo de compatibilidad de Game.exe (Windows 11: Fog.dll).
//
// Reemplaza al .reg estático, que se desactualizaba cuando cambiaba la IP.
func ConfigurarRegistro(carpetaJuego string) error {
	bn, _, err := registry.CreateKey(registry.CURRENT_USER, ClaveBattleNet, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer bn.Close()
	actual, _, err := bn.GetStringsValue(ValorGateways)
	if err != nil && err != registry.ErrNotExist {
		actual = nil
	}
	if err := bn.SetStringsValue(ValorGateways, MezclarGateways(actual, IPRealm, ZonaRealm, NombreRealm)); err != nil {
		return err
	}

	d2, _, err := registry.CreateKey(registry.CURRENT_USER, ClaveDiablo, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer d2.Close()
	if err := d2.SetStringValue("BNETIP", IPRealm); err != nil {
		return err
	}

	compat, _, err := registry.CreateKey(registry.CURRENT_USER, ClaveCompat, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer compat.Close()
	return compat.SetStringValue(filepath.Join(carpetaJuego, "Game.exe"), CompatGameExe)
}
