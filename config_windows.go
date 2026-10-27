//go:build windows

package main

import "golang.org/x/sys/windows/registry"

const claveConfig = `Software\OmbuServer\Launcher`

// leerConfigGuardada lee la configuración del registro; false si todavía no hay.
func leerConfigGuardada(c *Config) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, claveConfig, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	if v, _, err := k.GetStringValue("carpeta"); err == nil {
		c.Carpeta = v
	}
	if v, _, err := k.GetIntegerValue("ventanas"); err == nil {
		c.Ventanas = int(v)
	}
	if v, _, err := k.GetIntegerValue("demora"); err == nil {
		c.Demora = int(v)
	}
	if v, _, err := k.GetIntegerValue("ventana"); err == nil {
		c.Ventana = v != 0
	}
	if v, _, err := k.GetIntegerValue("sonido"); err == nil {
		c.Sonido = v != 0
	}
	if v, _, err := k.GetIntegerValue("idioma"); err == nil {
		c.Idioma = int(v)
	}
	if v, _, err := k.GetIntegerValue("escala"); err == nil {
		c.Escala = int(v)
	}
	if v, _, err := k.GetIntegerValue("vista"); err == nil {
		c.Vista = int(v)
	}
	if v, _, err := k.GetIntegerValue("vozModo"); err == nil {
		c.VozModo = int(v)
	}
	if v, _, err := k.GetIntegerValue("vozTecla"); err == nil {
		c.VozTecla = int(v)
	}
	if v, _, err := k.GetStringValue("vozTeclaNombre"); err == nil {
		c.VozTeclaNombre = v
	}
	if v, _, err := k.GetIntegerValue("vozSensibilidad"); err == nil {
		c.VozSensibilidad = int(v)
	}
	return true
}

func escribirConfigGuardada(c Config) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, claveConfig, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	b := func(x bool) uint32 {
		if x {
			return 1
		}
		return 0
	}
	for _, e := range []error{
		k.SetStringValue("carpeta", c.Carpeta),
		k.SetDWordValue("ventanas", uint32(c.Ventanas)),
		k.SetDWordValue("demora", uint32(c.Demora)),
		k.SetDWordValue("ventana", b(c.Ventana)),
		k.SetDWordValue("sonido", b(c.Sonido)),
		k.SetDWordValue("idioma", uint32(c.Idioma)),
		k.SetDWordValue("escala", uint32(c.Escala)),
		k.SetDWordValue("vista", uint32(c.Vista)),
		k.SetDWordValue("vozModo", uint32(c.VozModo)),
		k.SetDWordValue("vozTecla", uint32(c.VozTecla)),
		k.SetStringValue("vozTeclaNombre", c.VozTeclaNombre),
		k.SetDWordValue("vozSensibilidad", uint32(c.VozSensibilidad)),
	} {
		if e != nil {
			return e
		}
	}
	return nil
}
