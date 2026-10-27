package main

// Pestaña TRADES (2026-10-13): los ítems que los jugadores dejan en la página de trade del alijo.
// Los datos los sirve el VPS web (nunca el servidor de juego):
//   - <contenido>/trades/datos.json: nombres (es/en), categoría, tamaño e imagen de cada ítem (cambia poco: se pide
//     una vez por sesión);
//   - <web>/trades-api/buscar: la búsqueda con filtros y páginas;
//   - <contenido>/trades/img/<clave>.png: las imágenes (las guarda la caché de la ventana).

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"ombuserver-launcher/ombu"
)

var (
	tradesDatosMu sync.Mutex
	tradesDatos   string
)

// TradesDatos devuelve datos.json (una vez por sesión).
func (a *App) TradesDatos() (string, error) {
	tradesDatosMu.Lock()
	defer tradesDatosMu.Unlock()
	if tradesDatos != "" {
		return tradesDatos, nil
	}
	cli := &http.Client{Timeout: 30 * time.Second}
	r, err := cli.Get(ombu.URLContenido + "trades/datos.json")
	if err != nil {
		return "", fmt.Errorf("no se pudo conectar con OmbuServer")
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return "", fmt.Errorf("el servidor respondió %d", r.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		return "", err
	}
	tradesDatos = string(b)
	return tradesDatos, nil
}

// TradesImagenes es la dirección base de las imágenes de los ítems.
func (a *App) TradesImagenes() string {
	return ombu.URLContenido + "trades/img/"
}

// TradesBuscar consulta la API de TRADES: categoría ("" = todas), calidad ("" = todas; normal, superior, magico,
// raro, unico, set, artesanal, runica), texto, página e idioma ("es"/"en").
// Devuelve el JSON tal cual (total, pag, por, porCalidad, items).
func (a *App) TradesBuscar(cat, cal, q string, pag int, lng string) (string, error) {
	v := url.Values{}
	if cat != "" {
		v.Set("cat", cat)
	}
	if cal != "" {
		v.Set("cal", cal)
	}
	if q = strings.TrimSpace(q); q != "" {
		v.Set("q", q)
	}
	v.Set("pag", fmt.Sprint(pag))
	v.Set("por", "24") // 3 filas de 8 (pedido del usuario, 2026-10-14)
	v.Set("lng", lng)
	cli := &http.Client{Timeout: 20 * time.Second}
	r, err := cli.Get(urlVoz + "trades-api/buscar?" + v.Encode())
	if err != nil {
		return "", fmt.Errorf("no se pudo conectar con OmbuServer")
	}
	defer r.Body.Close()
	if r.StatusCode == http.StatusTooManyRequests {
		return "", fmt.Errorf("demasiadas búsquedas seguidas: esperá un momento")
	}
	if r.StatusCode != http.StatusOK {
		return "", fmt.Errorf("el servidor respondió %d", r.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	return string(b), err
}
