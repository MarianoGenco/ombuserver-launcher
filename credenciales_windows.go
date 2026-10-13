//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Cuenta y contraseña del chat recordadas: en HKCU\Software\OmbuServer\Launcher ("chatCuenta" y "chatClave"). La
// contraseña va cifrada con DPAPI (CryptProtectData): solo la puede descifrar el mismo usuario de Windows en esta
// PC. Nunca se guarda en texto plano.

func protegerDPAPI(datos []byte, descifrar bool) ([]byte, error) {
	if len(datos) == 0 {
		return nil, nil
	}
	in := windows.DataBlob{Size: uint32(len(datos)), Data: &datos[0]}
	var out windows.DataBlob
	var err error
	if descifrar {
		err = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}

func guardarCredenciales(cuenta, clave string) {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, claveConfig, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	k.SetStringValue("chatCuenta", cuenta)
	if cif, err := protegerDPAPI([]byte(clave), false); err == nil {
		k.SetBinaryValue("chatClave", cif)
	}
}

func guardarSoloCuenta(cuenta string) {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, claveConfig, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	k.SetStringValue("chatCuenta", cuenta)
	k.DeleteValue("chatClave")
}

func borrarCredenciales() {
	k, err := registry.OpenKey(registry.CURRENT_USER, claveConfig, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	k.DeleteValue("chatCuenta")
	k.DeleteValue("chatClave")
}

// leerCredenciales: ok si hay una cuenta recordada; la clave viene vacía si solo se recordó la cuenta.
func leerCredenciales() (cuenta, clave string, ok bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, claveConfig, registry.QUERY_VALUE)
	if err != nil {
		return "", "", false
	}
	defer k.Close()
	cuenta, _, err = k.GetStringValue("chatCuenta")
	if err != nil || cuenta == "" {
		return "", "", false
	}
	if cif, _, err := k.GetBinaryValue("chatClave"); err == nil {
		if plano, err := protegerDPAPI(cif, true); err == nil {
			clave = string(plano)
		}
	}
	return cuenta, clave, true
}

// ultimaCuentaBnet es la última cuenta con la que entró el juego a Battle.net ("Last BNet").
func ultimaCuentaBnet() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Blizzard Entertainment\Diablo II`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, _ := k.GetStringValue("Last BNet")
	return v
}
