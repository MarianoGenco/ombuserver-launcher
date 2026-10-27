"use strict";

// Pestaña TRADES (2026-10-13): los ítems que los jugadores dejan en la página de trade del alijo (botón TRADE del
// alijo en el juego). Los datos vienen del VPS web por el lado Go (trades.go): datos.json (nombres, categorías e
// imágenes, una vez por sesión) y la búsqueda (filtro, texto y página). Las imágenes son las de Resurrected.
// Los nombres de ítems, jugadores y propiedades van con translate="no": la API ya los manda en el idioma elegido
// (las propiedades, por ahora, en inglés: como las tiene la tabla del juego que lee el servidor).

const trades = { cat: "", cal: "", q: "", pag: 1, total: 0, por: 24, datos: null, imagenes: "", pedido: 0, ultimo: "",
  porCalidad: {} };
// Con la pestaña abierta se actualiza sola cada minuto (y con el botón ↻).
const TRADES_REFRESCO_MS = 60000;

// Colores de calidad como en el juego.
const COLOR_CALIDAD = { 1: "#8a8a8a", 2: "#e8e6e1", 3: "#e8e6e1", 4: "#7f7fff", 5: "#2ed12e", 6: "#ffff64", 7: "#c7b377", 8: "#ff9a1f" };
const COLOR_RUNICA = "#c7b377";
// Hechizos y joyas con propiedades: en el juego no hay "superiores" de estos, así que van en azul como los mágicos
// (el Hechizo pequeño de calidad 3 salía blanco, 2026-10-14).
const CON_PODERES = ["cm1", "cm2", "cm3", "jew"];
function colorNombre(it) {
  if (it.rw !== undefined) return COLOR_RUNICA;
  if (CON_PODERES.includes(it.c) && (it.q || 2) <= 3 && (it.props || []).length) return COLOR_CALIDAD[4];
  return COLOR_CALIDAD[it.q] || COLOR_CALIDAD[2];
}
// Filtro por calidad (desplegable al lado del buscador, 2026-10-14): la clave es la que entiende la API (api.py
// CALIDADES) y el color el del nombre en el juego. Las rúnicas van aparte (en el juego son normales o superiores).
const CALIDADES = [
  ["", "Todos", ""], ["normal", "Normal", COLOR_CALIDAD[2]], ["superior", "Superior", COLOR_CALIDAD[3]],
  ["magico", "Mágico", COLOR_CALIDAD[4]], ["raro", "Raro", COLOR_CALIDAD[6]], ["set", "Set", COLOR_CALIDAD[5]],
  ["unico", "Único", COLOR_CALIDAD[7]], ["artesanal", "Artesanal", COLOR_CALIDAD[8]], ["runica", "Palabra rúnica", COLOR_RUNICA],
];

// Datos de ejemplo (solo fuera de Wails, para revisar el diseño en un navegador).
if (!enWails) Object.assign(api, {
  TradesDatos: async () => JSON.stringify({ codigos: { uap: { es: "Shako", en: "Shako", img: "" }, rin: { es: "Anillo", en: "Ring", img: "" },
    cm1: { es: "Hechizo pequeño", en: "Small Charm", img: "" }, "7cr": { es: "Espada fantasma", en: "Phase Blade", img: "" } }, unicos: {}, sets: {}, runicas: {} }),
  TradesImagenes: async () => "",
  TradesBuscar: async (cat, cal, q, pag) => {
    const todos = [
      { pj: "hechitest", cuenta: "Agos", enLinea: true, nombre: "Corona de Harlequín", cat: "cascos", item: { c: "uap", q: 7, u: 0, props: ["+2 a todas las habilidades"] } },
      { pj: "LuchoPaladin", cuenta: "lucho", enLinea: false, nombre: "Anillo de la Piedra", cat: "anillos", item: { c: "rin", q: 6, props: ["+1 a todas las habilidades"] } },
      { pj: "Arenados", cuenta: "arena2", enLinea: false, nombre: "Hechizo pequeño", cat: "hechizos", item: { c: "cm1", q: 4, props: ["+20 a la vida"] } },
      { pj: "Arenados", cuenta: "arena2", enLinea: false, nombre: "Grito", cat: "armas", item: { c: "7cr", q: 2, rw: 1, props: [] } },
    ];
    const de = (it) => (it.item.rw !== undefined ? "runica" : { 2: "normal", 3: "superior", 4: "magico", 5: "set", 6: "raro", 7: "unico", 8: "artesanal" }[it.item.q]);
    const porCalidad = {};
    todos.forEach((x) => { porCalidad[de(x)] = (porCalidad[de(x)] || 0) + 1; });
    const items = todos.filter((x) => (!cat || x.cat === cat) && (!cal || de(x) === cal));
    return JSON.stringify({ total: items.length, pag, por: 24, porCalidad, items });
  },
});

// Los susurros van a la cuenta (/w *Cuenta): abajo de cada ítem va la cuenta (sin el asterisco, pedido del
// usuario); si no se sabe, el personaje.
function dueno(x) { return x.cuenta || x.pj; }

// Color propio de cada dueño: el mismo siempre (sale del nombre).
function colorDueno(pj) {
  let h = 0;
  for (const c of pj.toLowerCase()) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return `hsl(${h % 360}, 70%, 64%)`;
}

function imagenDe(it) {
  const d = trades.datos;
  if (!d) return "";
  const base = d.codigos[it.c] || {};
  let clave = base.img;
  if (it.u !== undefined && d.unicos[it.u] && d.unicos[it.u].img) clave = d.unicos[it.u].img;
  else if (it.s !== undefined && d.sets[it.s] && d.sets[it.s].img) clave = d.sets[it.s].img;
  else if (it.pic !== undefined && ["rin", "amu", "cm1", "cm2", "cm3", "jew"].includes(it.c)) clave = `v_${it.c}_${it.pic}`;
  return clave ? `${trades.imagenes}${clave}.png` : "";
}

function nombreBase(it) {
  const b = trades.datos && trades.datos.codigos[it.c];
  return b ? b[idioma === "en" ? "en" : "es"] : it.c;
}

function el(tag, clase, texto) {
  const e = document.createElement(tag);
  if (clase) e.className = clase;
  if (texto !== undefined) e.textContent = texto;
  return e;
}

function pintarTrades(r, silencioso) {
  const grilla = $("trades-grilla");
  const scroll = grilla.scrollTop;
  grilla.textContent = "";
  trades.total = r.total;
  trades.porCalidad = r.porCalidad || {};
  pintarCalidades();
  const paginas = Math.max(1, Math.ceil(r.total / trades.por));
  $("trades-pagina").textContent = `${trades.pag} / ${paginas}`;
  $("trades-ant").disabled = trades.pag <= 1;
  $("trades-sig").disabled = trades.pag >= paginas;
  const total = $("trades-total");
  total.textContent = "";
  total.append(document.createTextNode(String(r.total)), document.createTextNode(" "), el("span", "", r.total === 1 ? "ítem" : "ítems"));
  $("trades-vacio").hidden = r.items.length > 0;
  if (!r.items.length) $("trades-vacio").textContent = trades.q || trades.cat || trades.cal ? "No hay ítems en trade con ese filtro." : "Todavía no hay ítems en trade.";
  for (const x of r.items) {
    const it = x.item;
    const tarjeta = el("div", "trades-item");
    const marco = el("div", "trades-img");
    const src = imagenDe(it);
    if (src) {
      const img = el("img");
      img.alt = "";
      img.loading = "lazy";
      img.src = src;
      const base = trades.datos && trades.datos.codigos[it.c] && trades.datos.codigos[it.c].img;
      img.onerror = () => { if (base && !img.src.endsWith(`/${base}.png`)) img.src = `${trades.imagenes}${base}.png`; else img.remove(); };
      marco.append(img);
    }
    const nombre = el("p", "trades-nombre", x.nombre);
    nombre.translate = false;
    nombre.style.color = colorNombre(it);
    // la cuenta, abajo de todo y separada por una línea: queda a la misma altura en todas las tarjetas
    const pie = el("p", "trades-dueno");
    const punto = el("span", x.enLinea ? "trades-punto en-linea" : "trades-punto");
    const nom = el("span", "", dueno(x));
    nom.translate = false;
    nom.style.color = colorDueno(x.cuenta || x.pj);
    pie.append(punto, nom);
    const cuerpo = el("div", "trades-centro");          // imagen y nombre, centrados arriba de la línea
    cuerpo.append(marco, nombre);
    tarjeta.append(cuerpo, pie);
    tarjeta.addEventListener("mouseenter", (e) => mostrarTip(x, e));
    tarjeta.addEventListener("mousemove", moverTip);
    tarjeta.addEventListener("mouseleave", () => { $("trades-tip").hidden = true; });
    // tocar el ítem abre el mensaje al dueño (mensajes.js)
    tarjeta.addEventListener("click", () => { $("trades-tip").hidden = true; abrirMensajeItem(x, src); });
    grilla.append(tarjeta);
  }
  grilla.scrollTop = silencioso ? scroll : 0;
}

function renglon(tip, etiqueta, valor, clase) {
  const p = el("p", clase || "");
  if (etiqueta) p.append(el("span", "trades-etiqueta", etiqueta), document.createTextNode(": "));
  const v = el("span", "", valor);
  v.translate = false;
  p.append(v);
  tip.append(p);
}

function mostrarTip(x, e) {
  const it = x.item;
  const tip = $("trades-tip");
  tip.textContent = "";
  const titulo = el("p", "trades-tip-titulo", x.nombre);
  titulo.translate = false;
  titulo.style.color = colorNombre(it);
  tip.append(titulo);
  if (it.u !== undefined || it.s !== undefined || it.rw !== undefined) {
    const b = el("p", "trades-tip-base", nombreBase(it));
    b.translate = false;
    tip.append(b);
  }
  if (it.eth) tip.append(el("p", "trades-tip-gris", "Etéreo"));
  if (it.noId) tip.append(el("p", "trades-tip-rojo", "Sin identificar"));
  if (it.def !== undefined) renglon(tip, "Defensa", String(it.def));
  if (it.dur) renglon(tip, "Durabilidad", `${it.dur[0]} / ${it.dur[1]}`);
  if (it.cant !== undefined) renglon(tip, "Cantidad", String(it.cant));
  if (it.huecos) renglon(tip, "Huecos", String(it.huecos));
  for (const p of it.props || []) renglon(tip, "", p, "trades-tip-prop");
  for (const p of it.propsRw || []) renglon(tip, "", p, "trades-tip-prop");
  for (const lista of it.propsSet || []) for (const p of lista) renglon(tip, "", p, "trades-tip-set");
  if (it.engarces && it.engarces.length) {
    tip.append(el("p", "trades-tip-sub", "Engarces"));
    for (const g of it.engarces) renglon(tip, "", nombreBase(g), "trades-tip-gris");
  }
  const pie = el("p", "trades-tip-pie");
  const nom = el("span", "", dueno(x));
  nom.translate = false;
  nom.style.color = colorDueno(x.cuenta || x.pj);
  pie.append(nom);
  if (x.cuenta) pie.append(el("span", "trades-tip-pj", ` · ${x.pj}`));
  tip.append(pie);
  tip.hidden = false;
  moverTip(e);
}

function moverTip(e) {
  const tip = $("trades-tip");
  const w = tip.offsetWidth, h = tip.offsetHeight;
  let x = e.clientX + 16, y = e.clientY + 12;
  if (x + w > 1016) x = e.clientX - w - 12;
  if (y + h > 592) y = Math.max(80, 592 - h);
  tip.style.left = `${x}px`;
  tip.style.top = `${y}px`;
}

// silencioso ("boton" o "auto"): no repinta si nada cambió y mantiene el scroll; la automática tampoco muestra los
// errores. Sin aviso de "Cargando…": ocupaba lugar abajo y achicaba las filas de golpe (pedido del usuario, 2026-10-14).
async function buscarTrades(silencioso) {
  const n = ++trades.pedido;
  try {
    if (!trades.datos) {
      trades.datos = JSON.parse(await api.TradesDatos());
      trades.imagenes = await api.TradesImagenes();
    }
    const texto = await api.TradesBuscar(trades.cat, trades.cal, trades.q, trades.pag, idioma === "en" ? "en" : "es");
    if (n !== trades.pedido || (silencioso && texto === trades.ultimo)) return;
    trades.ultimo = texto;
    const r = JSON.parse(texto);
    // si la página quedó vacía porque se sacaron ítems, ir a la última que tenga
    const paginas = Math.max(1, Math.ceil(r.total / trades.por));
    if (trades.pag > paginas) { trades.pag = paginas; return buscarTrades(silencioso); }
    pintarTrades(r, silencioso);
  } catch (err) {
    if (n !== trades.pedido || silencioso === "auto") return;
    $("trades-grilla").textContent = "";
    $("trades-vacio").hidden = false;
    $("trades-vacio").textContent = String(err && err.message || err);
  }
}

function abrirTrades() {
  $("opciones").hidden = true;
  $("principal").hidden = true;
  $("chat").hidden = true;
  $("trades").hidden = false;
  marcarMenu("trades");
  buscarTrades();
}

document.querySelectorAll(".trades-cat").forEach((b) => b.addEventListener("click", () => {
  document.querySelectorAll(".trades-cat").forEach((o) => o.classList.toggle("activa", o === b));
  trades.cat = b.dataset.cat;
  trades.pag = 1;
  buscarTrades();
}));
// ---------- desplegable de calidad ----------
// Cada opción con su color y cuántos hay (con la categoría y el texto de ahora). Se abre con el botón, se elige con
// el mouse o con las flechas y Enter, y se cierra con Escape o tocando afuera.
function pintarCalidades() {
  const lista = $("trades-calidad-lista");
  lista.textContent = "";
  let total = 0;
  for (const k in trades.porCalidad) total += trades.porCalidad[k];
  for (const [clave, nombre, color] of CALIDADES) {
    const n = clave ? (trades.porCalidad[clave] || 0) : total;
    const op = el("button", "desplegable-opcion");
    op.type = "button";
    op.setAttribute("role", "option");
    op.dataset.cal = clave;
    op.classList.toggle("elegida", clave === trades.cal);
    op.classList.toggle("vacia", !n && clave !== trades.cal);
    op.setAttribute("aria-selected", clave === trades.cal ? "true" : "false");
    const punto = el("span", "desplegable-punto");
    if (color) { punto.style.background = color; punto.style.boxShadow = `0 0 6px ${color}`; } else punto.classList.add("todos");
    const nom = el("span", "desplegable-nombre", nombre);
    if (color) nom.style.color = color;
    op.append(punto, nom, el("span", "desplegable-cuantos", String(n)));
    op.addEventListener("click", () => elegirCalidad(clave));
    lista.append(op);
  }
  const [, nombre, color] = CALIDADES.find((c) => c[0] === trades.cal) || CALIDADES[0];
  const texto = $("trades-calidad-texto");
  texto.textContent = trades.cal ? nombre : "Calidad";
  texto.style.color = color || "";
  const punto = $("trades-calidad-punto");
  punto.hidden = !trades.cal;
  punto.style.background = color || "";
  punto.style.boxShadow = color ? `0 0 6px ${color}` : "";
  $("trades-calidad").classList.toggle("con-filtro", !!trades.cal);
}

function abrirCalidades(abrir) {
  const lista = $("trades-calidad-lista");
  if (abrir === undefined) abrir = lista.hidden;
  lista.hidden = !abrir;
  $("trades-calidad").classList.toggle("abierto", abrir);
  $("trades-calidad-boton").setAttribute("aria-expanded", abrir ? "true" : "false");
  if (abrir) (lista.querySelector(".elegida") || lista.firstChild)?.focus();
}

function elegirCalidad(clave) {
  abrirCalidades(false);
  $("trades-calidad-boton").focus();
  if (clave === trades.cal) return;
  trades.cal = clave;
  trades.pag = 1;
  pintarCalidades();
  buscarTrades();
}

$("trades-calidad-boton").addEventListener("click", () => abrirCalidades());
$("trades-calidad-lista").addEventListener("keydown", (e) => {
  const ops = [...$("trades-calidad-lista").children];
  const i = ops.indexOf(document.activeElement);
  if (e.key === "ArrowDown" || e.key === "ArrowUp") {
    e.preventDefault();
    ops[(i + (e.key === "ArrowDown" ? 1 : ops.length - 1)) % ops.length].focus();
  } else if (e.key === "Escape" || e.key === "Tab") {
    e.preventDefault();
    abrirCalidades(false);
    $("trades-calidad-boton").focus();
  }
});
$("trades-calidad-boton").addEventListener("keydown", (e) => {
  if (e.key === "ArrowDown") { e.preventDefault(); abrirCalidades(true); }
});
document.addEventListener("mousedown", (e) => {
  if (!$("trades-calidad-lista").hidden && !$("trades-calidad").contains(e.target)) abrirCalidades(false);
});
pintarCalidades();

let tradesEspera = null;
$("trades-buscar").addEventListener("input", () => {
  clearTimeout(tradesEspera);
  tradesEspera = setTimeout(() => { trades.q = $("trades-buscar").value; trades.pag = 1; buscarTrades(); }, 350);
});
$("trades-ant").addEventListener("click", () => { if (trades.pag > 1) { trades.pag--; buscarTrades(); } });
$("trades-sig").addEventListener("click", () => { trades.pag++; buscarTrades(); });
$("trades-refrescar").addEventListener("click", () => {
  const b = $("trades-refrescar");
  b.classList.remove("girando");
  void b.offsetWidth;
  b.classList.add("girando");
  buscarTrades("boton");
});
// Actualización automática: solo con la pestaña a la vista, la ventana visible y sin el detalle de un ítem abierto
// (repintar le sacaría el ítem de abajo del mouse).
setInterval(() => {
  if ($("trades").hidden || document.hidden || !$("trades-tip").hidden || !$("trades-calidad-lista").hidden || !$("msj-modal").hidden) return;
  buscarTrades("auto");
}, TRADES_REFRESCO_MS);
