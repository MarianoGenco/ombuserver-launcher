"use strict";

// Fuera de Wails (abriendo index.html en un navegador para revisar el diseño) se usan datos de
// ejemplo; dentro del launcher, window.go y window.runtime los inyecta Wails.
const enWails = !!(window.go && window.go.main);
const api = enWails ? window.go.main.App : simulacion();

const $ = (id) => document.getElementById(id);
let estado = null;
let trabajando = false;
let baneado = false;
let cargando = true;
let jugandoMudo = false;  // JUGAR en curso sin mostrar nada en el boton      // al abrir: el boton muestra la carga hasta saber el estado real (juego + baneo)      // HWID baneado: el boton queda en rojo "BANEADO" y no deja jugar

function mostrarEstado(e) {
  estado = e;
  $("version").textContent = "v" + e.version;
  $("carpeta-ruta").textContent = e.config.carpeta || "Elegir…";
  $("carpeta").title = e.config.carpeta || "Elegir la carpeta del juego";
  if (!trabajando) {
    aviso(e.aviso, e.mensaje);
    botonNormal();
  }
}

function aviso(tipo, texto) {
  $("estado").dataset.aviso = tipo;
  $("estado-texto").textContent = texto;
}

let avisoActualizado = true;
let ultimaRevision = 0;

// silencioso: sin el cartel "Revisando…" (al volver a la ventana; molestaba cada vez que se movía el launcher)
async function revisar(silencioso) {
  ultimaRevision = Date.now();
  if (!silencioso) aviso("trabajando", "Revisando…");
  try {
    const e = await api.Estado();
    if (e.recien && avisoActualizado && e.aviso === "ok") {
      avisoActualizado = false;
      e.mensaje = "Launcher actualizado";
    }
    mostrarEstado(e);
  } catch (err) {
    aviso("error", String(err));
  }
}

async function elegirCarpeta() {
  try {
    mostrarEstado(await api.ElegirCarpeta());
  } catch (err) {
    aviso("error", String(err));
  }
}

// ---------- JUGAR ----------
function botonTrabajando(texto, porcentaje) {
  terminarCarga(true);
  const b = $("jugar");
  b.classList.add("trabajando");
  b.disabled = true;
  $("jugar-texto").textContent = porcentaje >= 0 ? `${texto.replace(/…$/, "")} ${porcentaje}%` : texto;
  $("jugar-relleno").style.width = (porcentaje >= 0 ? porcentaje : 0) + "%";
  aviso("trabajando", texto);
}

function botonNormal() {
  if (cargando) return;   // todavia no se sabe el estado: sigue la animacion de carga
  if (baneado) { botonBaneado(); return; }
  const b = $("jugar");
  const accion = estado ? estado.accion : "";
  b.classList.remove("trabajando");
  b.classList.toggle("actualizar", accion === "actualizar" || accion === "instalar");
  $("jugar-texto").textContent = { actualizar: "Actualizar", instalar: "Instalar LoD" }[accion] || "Jugar";
  $("jugar-relleno").style.width = "0";
  b.disabled = !accion;
}

async function actualizar() {
  if (trabajando) return;
  trabajando = true;
  botonTrabajando("Buscando actualizaciones…", -1);
  try {
    const reinicia = await api.Actualizar();
    if (reinicia) return; // el launcher nuevo se abre solo y este se cierra
    trabajando = false;
    await revisar();
  } catch (err) {
    trabajando = false;
    botonNormal();
    const m = String(err);
    aviso("error", m.charAt(0).toUpperCase() + m.slice(1));
  }
}

async function instalar() {
  if (trabajando) return;
  trabajando = true;
  botonTrabajando("Preparando la instalación…", -1);
  try {
    await api.Instalar();
    trabajando = false;
    await revisar();
  } catch (err) {
    trabajando = false;
    botonNormal();
    const m = String(err);
    aviso("error", m.charAt(0).toUpperCase() + m.slice(1));
  }
}

function botonGrande() {
  if (baneado) return;
  const accion = estado ? estado.accion : "";
  if (accion === "actualizar") actualizar(); else if (accion === "instalar") instalar(); else jugar();
}

async function jugar() {
  if (trabajando) return;
  trabajando = true;
  jugandoMudo = true;   // el boton no cambia: solo la animacion de apretar (los pasos internos no se muestran)
  try {
    await api.Jugar();
    trabajando = false;
    jugandoMudo = false;
    botonNormal();
    setTimeout(revisar, 8000);
  } catch (err) {
    trabajando = false;
    jugandoMudo = false;
    botonNormal();
    const m = String(err);
    if (m.toUpperCase().includes("BANEADO")) {
      mostrarBaneado(m.replace(/.*BANEADO:?\s*/i, ""));
      return;
    }
    aviso("error", m.charAt(0).toUpperCase() + m.slice(1));
  }
}

// ---------- BANEADO ----------
// Solo el boton grande: rojo, letras blancas, "BANEADO", deshabilitado. El motivo va en el tooltip.
let motivoBaneo = "";
function botonBaneado() {
  const b = $("jugar");
  b.classList.remove("trabajando", "actualizar");
  b.classList.add("baneado");
  $("jugar-texto").textContent = "BANEADO";
  $("jugar-relleno").style.width = "0";
  b.disabled = true;
  b.title = motivoBaneo;   // solo si hay un motivo real
}
function mostrarBaneado(motivo) {
  baneado = true;
  motivoBaneo = motivo || "";
  if (!cargando) botonBaneado();
}

// Fin de la carga inicial: el boton aparece directo en su estado correcto, con una entrada suave.
function terminarCarga(sinPintar) {
  if (!cargando) return;
  cargando = false;
  const b = $("jugar");
  b.classList.remove("cargando");
  b.classList.add("aparecer");
  setTimeout(() => b.classList.remove("aparecer"), 400);
  if (!sinPintar) botonNormal();
}

async function verificarBaneo() {
  if (!api.EstadoBaneo) return;
  try {
    const b = await api.EstadoBaneo();
    if (b && b.baneado) mostrarBaneado(b.motivo);
  } catch (e) { /* si el servidor no responde, no se bloquea (el candado real es del servidor del juego) */ }
}

function alProgreso(p) {
  if (!trabajando) return;
  if (jugandoMudo && !(p.porcentaje >= 0)) return;   // al jugar solo se muestra una descarga real (con porcentaje)
  botonTrabajando(p.texto, p.porcentaje);
}

// ---------- novedades ----------
async function cargarNoticias() {
  let lista = [];
  try { lista = await api.Noticias(); } catch (_) { /* queda vacío */ }
  const cont = $("noticias");
  cont.textContent = "";
  // noticias.json puede traer la versión en inglés (title_en, text_en); si no está, va la original
  const en = (n, campo) => (idioma === "en" && n[campo + "_en"]) || n[campo];
  for (const n of lista) {
    const art = document.createElement("article");
    art.className = "noticia";
    const h = document.createElement("h3");
    if (n.link) {
      const a = document.createElement("a");
      a.textContent = en(n, "title");
      a.addEventListener("click", () => api.AbrirEnlace(n.link));
      h.append(a);
    } else {
      h.textContent = en(n, "title");
    }
    art.append(h);
    if (n.date) {
      const f = document.createElement("div");
      f.className = "fecha";
      f.textContent = n.date;
      art.append(f);
    }
    const p = document.createElement("p");
    p.textContent = en(n, "text");
    art.append(p);
    cont.append(art);
  }
}

// ---------- configuración (se guarda sola al cambiar algo) ----------
const opciones = { ventanas: 1, demora: 3, ventana: true, sonido: true, idioma: 1, escala: 100, vista: 0 };
let ocultarGuardado = null;

function pintarOpciones() {
  const c = estado ? estado.config : null;
  $("op-carpeta").textContent = (c && c.carpeta) || "Sin elegir";
  const nota = $("op-carpeta-estado");
  if (estado && estado.juegoOk) { nota.dataset.aviso = "ok"; nota.textContent = "Diablo II 1.14b encontrado."; }
  else if (estado) { nota.dataset.aviso = "carpeta"; nota.textContent = estado.aviso === "carpeta" ? estado.mensaje : ""; }
  $("op-ventanas").textContent = opciones.ventanas;
  $("op-demora").textContent = opciones.demora;
  $("op-ventana").setAttribute("aria-checked", opciones.ventana ? "true" : "false");
  $("op-sonido").setAttribute("aria-checked", opciones.sonido ? "true" : "false");
  document.querySelectorAll("#op-idioma button").forEach((b) => b.classList.toggle("activo", Number(b.dataset.idioma) === opciones.idioma));
  document.querySelectorAll("#op-escala button").forEach((b) => b.classList.toggle("activo", Number(b.dataset.escala) === opciones.escala));
  $("campo-escala").classList.toggle("deshabilitado", !opciones.ventana);   // solo en modo ventana (mod_vista.cpp)
  $("campo-demora").classList.toggle("deshabilitado", opciones.ventanas < 2);
}

async function guardarOpciones() {
  pintarOpciones();
  try {
    const c = await api.GuardarOpciones(opciones.ventanas, opciones.demora, opciones.ventana, opciones.sonido, opciones.idioma, opciones.escala, opciones.vista);
    if (estado) estado.config = c;
    const g = $("guardado");
    g.classList.add("visible");
    clearTimeout(ocultarGuardado);
    ocultarGuardado = setTimeout(() => g.classList.remove("visible"), 1500);
  } catch (err) {
    aviso("error", String(err));
  }
}

// Idioma del launcher (Acerca de; la traducción la hace i18n.js). El lado Go lo usa para el diálogo de la carpeta y
// para que Battle.net conteste en ese idioma desde la próxima conexión del chat.
function avisarIdiomaAlGo() { if (api.UsarIdioma) api.UsarIdioma(idioma).catch(() => {}); }
document.querySelectorAll("#op-idioma-launcher button").forEach((b) => b.addEventListener("click", () => {
  if (b.dataset.idiomaLauncher === idioma) return;
  cambiarIdioma(b.dataset.idiomaLauncher);
  const g = $("guardado");
  g.classList.add("visible");
  clearTimeout(ocultarGuardado);
  ocultarGuardado = setTimeout(() => g.classList.remove("visible"), 1500);
}));
document.addEventListener("idioma", () => {
  avisarIdiomaAlGo();
  if (typeof pintarUsuarios === "function") pintarUsuarios();
  cargarNoticias();
});
avisarIdiomaAlGo();

async function cargarInfo() {
  try {
    const i = await api.Info();
    $("ac-version").textContent = "v" + i.version;
    $("ac-servidor").textContent = i.servidor;
  } catch (_) { /* queda con guiones */ }
}

function abrirOpciones() {
  if (estado) Object.assign(opciones, {
    ventanas: estado.config.ventanas, demora: estado.config.demora, ventana: estado.config.ventana,
    sonido: estado.config.sonido, idioma: estado.config.idioma, escala: estado.config.escala || 100,
    vista: estado.config.vista || 0,
  });
  pintarOpciones();
  vozPintarOpciones();
  cargarInfo();
  $("opciones").hidden = false;
  $("principal").hidden = true;
  $("chat").hidden = true;
  $("trades").hidden = true;
  marcarMenu("opciones");
}

function cerrarOpciones() {
  $("opciones").hidden = true;
  $("principal").hidden = false;
  $("chat").hidden = true;
  $("trades").hidden = true;
  marcarMenu("inicio");
}

function marcarMenu(vista) {
  document.querySelectorAll(".menu-item").forEach((b) => b.classList.toggle("activo", b.dataset.vista === vista));
}

function mostrarSeccion(nombre) {
  document.querySelectorAll(".seccion").forEach((b) => b.classList.toggle("activa", b.dataset.seccion === nombre));
  document.querySelectorAll(".grupo").forEach((g) => { g.hidden = g.dataset.seccion !== nombre; });
}
document.querySelectorAll(".seccion").forEach((b) => b.addEventListener("click", () => mostrarSeccion(b.dataset.seccion)));

const limites = { ventanas: [1, 8], demora: [0, 30] };
document.querySelectorAll(".contador").forEach((c) => {
  const campo = c.dataset.campo;
  c.querySelector(".menos").addEventListener("click", () => {
    const v = Math.max(limites[campo][0], opciones[campo] - 1);
    if (v !== opciones[campo]) { opciones[campo] = v; guardarOpciones(); }
  });
  c.querySelector(".mas").addEventListener("click", () => {
    const v = Math.min(limites[campo][1], opciones[campo] + 1);
    if (v !== opciones[campo]) { opciones[campo] = v; guardarOpciones(); }
  });
});

$("op-ventana").addEventListener("click", () => { opciones.ventana = !opciones.ventana; guardarOpciones(); });
$("op-sonido").addEventListener("click", () => { opciones.sonido = !opciones.sonido; guardarOpciones(); });
document.querySelectorAll("#op-escala button").forEach((b) => b.addEventListener("click", () => {
  const v = Number(b.dataset.escala);
  // 800×600 = vista normal; 1200×900 = vista alejada (la partida a 1200×900, mod_vista.cpp)
  if (v !== opciones.escala) { opciones.escala = v; opciones.vista = v === 150 ? 1 : 0; guardarOpciones(); }
}));
document.querySelectorAll("#op-idioma button").forEach((b) => b.addEventListener("click", () => {
  const v = Number(b.dataset.idioma);
  if (v !== opciones.idioma) { opciones.idioma = v; guardarOpciones(); }
}));
$("op-cambiar").addEventListener("click", async () => { await elegirCarpeta(); pintarOpciones(); cargarInfo(); });
$("op-abrir-juego").addEventListener("click", () => api.AbrirCarpeta("juego"));
document.addEventListener("keydown", (e) => { if (e.key === "Escape" && !$("opciones").hidden) cerrarOpciones(); });

document.querySelectorAll(".menu-item").forEach((b) => b.addEventListener("click", () => {
  if (b.dataset.vista === "opciones") abrirOpciones();
  else if (b.dataset.vista === "chat") abrirChat();
  else if (b.dataset.vista === "trades") abrirTrades();   // trades.js
  else cerrarOpciones();
}));

$("carpeta").addEventListener("click", elegirCarpeta);
$("jugar").addEventListener("click", botonGrande);
$("minimizar").addEventListener("click", () => enWails && window.runtime.WindowMinimise());
$("cerrar").addEventListener("click", () => enWails ? window.runtime.Quit() : window.close());
document.addEventListener("contextmenu", (e) => e.preventDefault());

if (enWails) window.runtime.EventsOn("progreso", alProgreso);
// Al abrir se espera el estado del juego y el del baneo antes de mostrar el boton (antes pasaba por "Instalar LoD"
// y despues "BANEADO"). El baneo espera hasta 3 s; si llega mas tarde, igual cambia el boton.
Promise.allSettled([
  revisar(),
  Promise.race([verificarBaneo(), new Promise((r) => setTimeout(r, 3000))]),
  new Promise((r) => setTimeout(r, 450)),   // minimo, para que la animacion no sea un parpadeo
]).then(() => terminarCarga());

// Si falta o cambia algo en la carpeta (por ejemplo, se borró un archivo), el botón se actualiza al instante.
let firmaCarpeta = null;
async function chequearCarpeta() {
  if (trabajando || !api.ChequeoRapido) return;
  try {
    const f = await api.ChequeoRapido();
    if (firmaCarpeta !== null && f !== firmaCarpeta) revisar();
    firmaCarpeta = f;
  } catch (_) { /* se reintenta en el proximo */ }
}
setInterval(chequearCarpeta, 3000);
// Al volver a la ventana se revisa en silencio y como mucho cada 30 s (mover la ventana también le devuelve el foco).
window.addEventListener("focus", () => {
  if (trabajando || Date.now() - ultimaRevision < 30000) return;
  firmaCarpeta = null;
  revisar(true);
});
cargarNoticias();

// ---------- chat de Battle.net (pestaña CHAT) ----------
var oyentes = {};
function escuchar(nombre, f) {
  if (enWails) window.runtime.EventsOn(nombre, f);
  else (oyentes[nombre] = oyentes[nombre] || []).push(f);
}
function emitir(nombre, dato) { (oyentes[nombre] || []).forEach((f) => f(dato)); }

// privado: la conversación de mensajes abierta en lugar del canal (mensajes.js), o null
const chat = { estado: null, usuarios: new Map(), canal: "", susurrando: "", reignorar: null, ocultarRespuestas: 0, privado: null };
// Rangos de Battle.net (bits de flags de PvPGN) -> figura del lobby (img/chat/rango-*.png), en el orden en que los
// mira el juego (Game.exe 1.14b, FUN_00442340): con un rango se dibuja esa figura en lugar del personaje.
const FLAGS = [[0x02, "operador"], [0x08, "moderador"], [0x01, "admin"], [0x04, "voz"]];
const NOMBRE_RANGO = { admin: "Admin", moderador: "Operador", operador: "Operador del canal", voz: "Voz", chat: "Chat" };
const CLASES = { ama: "Amazona", sor: "Hechicera", nec: "Nigromante", pal: "Paladín", bar: "Bárbaro", dru: "Druida", ass: "Asesina" };

function abrirChat() {
  $("opciones").hidden = true;
  $("principal").hidden = true;
  $("chat").hidden = false;
  $("trades").hidden = true;
  marcarMenu("chat");
  if (!$("chat-sala").hidden) $("sala-texto").focus();
}

// Nombre a mostrar: solo el del personaje (sin título); si no está en Diablo II, la cuenta.
function nombreDe(usuario, cuenta) {
  const u = chat.usuarios.get(usuario);
  if (u && u.personaje && u.personaje.nombre) return u.personaje.nombre;
  return cuenta || (usuario || "").replace(/^.*\*/, "");
}

function pintarChat(e) {
  if (e) chat.estado = e;
  const st = chat.estado || {};
  const conectado = !!st.conectado;
  $("chat-login").hidden = conectado || st.pausado;
  $("chat-sala").hidden = !(conectado || st.pausado);
  $("sala-aviso").textContent = st.pausado ? "En pausa mientras jugás con esta cuenta" : (st.mensaje || "");
  if (!conectado && !st.pausado) {
    if (st.recordado && !$("login-cuenta").value) $("login-cuenta").value = st.recordado;
    $("login-entrar").disabled = !!st.conectando;
    $("login-entrar").textContent = st.conectando ? "Entrando…" : "Entrar";
    if (st.mensaje) loginMensaje(st.mensaje);
  }
}

function loginMensaje(t, ok) {
  $("login-mensaje").textContent = t || "";
  $("login-mensaje").classList.toggle("ok", !!ok);
}

async function entrarChat(cuenta, clave) {
  loginMensaje("");
  try {
    pintarChat(await api.ChatEntrar(cuenta, clave, $("login-recordar").checked));
    $("login-clave").value = "";
  } catch (err) {
    pintarChat(chat.estado);
    loginMensaje(String(err));
  }
}

$("login-form").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  const cuenta = $("login-cuenta").value.trim(), clave = $("login-clave").value;
  if (!cuenta || !clave) { loginMensaje("Completá la cuenta y la contraseña"); return; }
  entrarChat(cuenta, clave);
});
$("sala-salir").addEventListener("click", async () => { await api.ChatSalir(); chat.usuarios.clear(); $("sala-mensajes").textContent = ""; pintarUsuarios(); mensajesCerrados(); });

// delServidor: el texto lo manda Battle.net (ya viene en el idioma del launcher) y no se traduce; lo que escribe la
// gente (msj-texto) tampoco.
// Colores de Diablo II para los códigos ÿc0..ÿc; (los anuncios de torneo vienen en violeta, ÿc;)
const COLORES_D2 = { "0": "#e8e6e1", "1": "#ff4d4d", "2": "#2ed12e", "3": "#7f7fff", "4": "#c7b377", "5": "#8a8a8a",
  "6": "#3a3a3a", "7": "#b8a87a", "8": "#ff9a1f", "9": "#ffff64", ":": "#1d8c1d", ";": "#c35bff" };

// caja: dónde se escribe (el canal, o la conversación privada de mensajes.js). Devuelve el renglón.
function agregarMensaje(clase, partes, delServidor, caja) {
  caja = caja || $("sala-mensajes");
  const abajo = caja.scrollHeight - caja.scrollTop - caja.clientHeight < 30;
  const p = document.createElement("div");
  p.className = "msj " + clase;
  if (delServidor) p.translate = false;
  for (const [c, t] of partes) {
    // códigos de color de Diablo II en el texto ("ÿc;" violeta, "ÿc4" dorado...): un tramo por color
    const tramos = String(t).split(/ÿc(.)/);
    for (let k = 0; k < tramos.length; k += 2) {
      if (!tramos[k]) continue;
      const s = document.createElement("span");
      if (c) s.className = c;
      if (c === "msj-texto") s.translate = false;
      if (k > 0 && COLORES_D2[tramos[k - 1]]) s.style.color = COLORES_D2[tramos[k - 1]];
      s.textContent = tramos[k];
      p.append(s);
    }
  }
  caja.append(p);
  while (caja.children.length > 400) caja.firstChild.remove();
  if (abajo) caja.scrollTop = caja.scrollHeight;
  return p;
}

function pintarUsuarios() {
  const cont = $("sala-usuarios");
  cont.textContent = "";
  $("sala-cantidad").textContent = chat.usuarios.size;
  for (const u of chat.usuarios.values()) {
    const d = document.createElement("div");
    d.className = "persona";
    const pj = u.personaje;
    const nombre = pj && pj.nombre ? pj.nombre : u.cuenta;
    d.title = pj && pj.clase
      ? `${pj.nombre} · ${T(CLASES[pj.clase] || "")}${pj.nivel ? " " + T("nivel " + pj.nivel) : ""} · ${T("cuenta " + u.cuenta)}`
      : `${u.cuenta} (chat)`;
    const fig = document.createElement("img");
    fig.alt = "";
    fig.className = "figura";
    const conRango = FLAGS.find(([bit]) => u.flags & bit);
    if (conRango || !(pj && pj.clase)) {
      // como en el lobby del juego: el rango manda sobre el personaje; sin rango ni personaje, la figura del chat
      const rango = conRango ? conRango[1] : "chat";
      fig.src = `img/chat/rango-${rango}.png`;
      d.title = `${T(NOMBRE_RANGO[rango])} · ` + d.title;
    } else {
      fig.src = `img/chat/clase-${pj.clase}.png`;
    }
    const n = document.createElement("span");
    n.className = "nombre";
    n.textContent = nombre;
    d.append(fig, n);
    if (u.flags & IGNORADO) {
      // la cruz roja del lobby del juego sobre el ignorado (img/chat/ignorado.png, 15 cuadros en bucle)
      const cruz = document.createElement("div");
      cruz.className = "cruz-ignorado";
      d.append(cruz);
      d.title = T("Ignorado") + " · " + d.title;
    }
    d.dataset.cuenta = (u.cuenta || "").replace(/^\*/, "").toLowerCase();
    if (chat.susurrando && chat.susurrando.toLowerCase() === (u.cuenta || "").toLowerCase()) d.classList.add("seleccionado");
    d.addEventListener("click", () => elegirSusurro(u.cuenta));
    d.addEventListener("contextmenu", (ev) => vozMenuPersona(ev, u));   // voz.js
    cont.append(d);
  }
  if (typeof vozMarcarTira === "function") vozMarcarTira();
  pintarFlechasTira();
}

// Flechas de la tira (como en el lobby): el par chico corre de a un personaje, el grande de a página (8).
function pintarFlechasTira() { /* siempre visibles, como en el juego */ }
const ANCHO_PERSONA = 74;   // .persona (70) + separación (4)
document.querySelectorAll(".tira-boton").forEach((b) => {
  const img = b.querySelector("img");
  const ver = (cuadro) => { img.src = `img/chat/tira-flecha-${cuadro}.png`; };
  b.addEventListener("mousedown", () => ver(b.dataset.apretada));
  b.addEventListener("mouseup", () => ver(b.dataset.reposo));
  b.addEventListener("mouseleave", () => ver(b.dataset.reposo));
  b.addEventListener("click", () => {
    const t = $("sala-usuarios");
    const paso = Number(b.dataset.paso);
    const ancho = Math.abs(paso) > 1 ? Math.max(ANCHO_PERSONA, Math.floor(t.clientWidth / ANCHO_PERSONA - 1) * ANCHO_PERSONA) : ANCHO_PERSONA;
    t.scrollBy({ left: Math.sign(paso) * ancho });
  });
});
// Arrastrar con el mouse corre la tira; si se movió más de 5 px no cuenta como clic (no elige para susurrar).
const arrastre = { activo: false, x: 0, inicio: 0, movio: false };
$("sala-usuarios").addEventListener("mousedown", (e) => {
  if (e.button !== 0) return;
  Object.assign(arrastre, { activo: true, x: e.clientX, inicio: $("sala-usuarios").scrollLeft, movio: false });
});
window.addEventListener("mousemove", (e) => {
  if (!arrastre.activo) return;
  const dx = e.clientX - arrastre.x;
  if (!arrastre.movio && Math.abs(dx) > 5) { arrastre.movio = true; $("sala-usuarios").classList.add("arrastrando"); }
  if (arrastre.movio) $("sala-usuarios").scrollLeft = arrastre.inicio - dx;
});
window.addEventListener("mouseup", () => {
  if (!arrastre.activo) return;
  arrastre.activo = false;
  $("sala-usuarios").classList.remove("arrastrando");
});
$("sala-usuarios").addEventListener("click", (e) => {
  if (arrastre.movio) { e.stopPropagation(); arrastre.movio = false; }
}, true);
$("sala-usuarios").addEventListener("dragstart", (e) => e.preventDefault());
// La rueda del mouse sobre la tira la corre de costado.
$("sala-usuarios").addEventListener("wheel", (e) => {
  if (Math.abs(e.deltaY) > Math.abs(e.deltaX)) { e.preventDefault(); $("sala-usuarios").scrollBy({ left: e.deltaY }); }
}, { passive: false });

// Tocar un personaje lo deja elegido para susurrarle: cada mensaje va con "/w *cuenta" y, al mandarlo, el prefijo
// vuelve a quedar escrito. Tocarlo otra vez (o elegir a otro) lo suelta.
const prefijoSusurro = (cuenta) => `/w *${cuenta} `;

function elegirSusurro(cuenta) {
  if (chat.privado) cerrarPrivado();   // con una conversación de mensajes abierta, se vuelve al canal (mensajes.js)
  const t = $("sala-texto");
  const previo = chat.susurrando;
  if (previo && t.value.startsWith(prefijoSusurro(previo))) t.value = t.value.slice(prefijoSusurro(previo).length);
  chat.susurrando = previo && previo.toLowerCase() === (cuenta || "").toLowerCase() ? "" : (cuenta || "");
  if (chat.susurrando) t.value = prefijoSusurro(chat.susurrando) + t.value;
  t.placeholder = chat.susurrando ? `Susurrando a ${nombreDe(chat.susurrando, chat.susurrando)}…` : "Escribí un mensaje…";
  pintarUsuarios();
  t.focus();
  t.setSelectionRange(t.value.length, t.value.length);
}

function soltarSusurro() {
  if (!chat.susurrando) return;
  if (chat.privado) {
    // conversación de mensajes abierta: se suelta en lo que quedó escrito para el canal, sin salir de la conversación
    const pre = prefijoSusurro(chat.susurrando);
    if (buzon.borrador.startsWith(pre)) buzon.borrador = buzon.borrador.slice(pre.length);
    chat.susurrando = "";
    pintarUsuarios();
    return;
  }
  elegirSusurro(chat.susurrando);
}

// Ignorar (/squelch): el servidor deja de mandarnos lo que esa cuenta escribe o susurra y la marca con el flag 0x20,
// que en la tira se ve con la cruz roja del juego. PvPGN se olvida de los ignorados al desconectarse: el launcher
// guarda la lista de cada cuenta y la vuelve a mandar al entrar al primer canal, sin mostrar las respuestas.
const IGNORADO = 0x20;
const RESPUESTA_IGNORAR = /( has been squelched| esta siendo ignorado)\.$|^(No such user|No existe tal usuario)\.$/;
const claveIgnorados = () => "chat-ignorados:" + ((chat.estado && chat.estado.usuario) || "").toLowerCase();
function ignoradosGuardados() {
  try { return JSON.parse(localStorage.getItem(claveIgnorados()) || "[]"); } catch (_) { return []; }
}
function recordarIgnorado(cuenta, si) {
  const c = (cuenta || "").replace(/^\*/, "").toLowerCase();
  if (!c || !(chat.estado && chat.estado.usuario)) return;
  const lista = ignoradosGuardados().filter((x) => x !== c);
  if (si) lista.push(c);
  try { localStorage.setItem(claveIgnorados(), JSON.stringify(lista)); } catch (_) { /* sin almacenamiento */ }
}
// Lo que se escribe a mano también cuenta: /squelch, /ignore, /unsquelch y /unignore.
function anotarComandoIgnorar(t) {
  const m = /^\/(un)?(squelch|ignore)\s+\*?(\S+)/i.exec(t);
  if (m) recordarIgnorado(m[3], !m[1]);
}
async function ignorar(cuenta, si) {
  const c = (cuenta || "").replace(/^\*/, "");
  const t = `/${si ? "squelch" : "unsquelch"} *${c}`;
  anotarComandoIgnorar(t);
  try { await api.ChatEnviar(t); } catch (err) { agregarMensaje("msj-error", [[null, String(err)]]); }
}
async function reaplicarIgnorados() {
  const yo = ((chat.estado && chat.estado.usuario) || "").toLowerCase();
  if (!yo || chat.reignorar === yo) return;
  chat.reignorar = yo;
  for (const c of ignoradosGuardados()) {
    chat.ocultarRespuestas++;
    try { await api.ChatEnviar(`/squelch *${c}`); } catch (_) { chat.ocultarRespuestas--; }
  }
  setTimeout(() => { chat.ocultarRespuestas = 0; }, 5000);   // si alguna respuesta no llegó, no tapar lo que siga
}
function respuestaOculta(ev) {
  if (chat.ocultarRespuestas > 0 && RESPUESTA_IGNORAR.test(ev.texto || "")) { chat.ocultarRespuestas--; return true; }
  return false;
}

function alEventoChat(ev) {
  switch (ev.tipo) {
    case "channel":
      reaplicarIgnorados();
      soltarSusurro();
      chat.usuarios.clear();
      chat.canal = ev.texto;
      $("sala-canal-nombre").textContent = ev.texto;
      agregarMensaje("msj-canal", [[null, `Entraste al canal: ${ev.texto}`]]);
      pintarUsuarios();
      vozCambioCanal(ev.texto);
      break;
    case "showuser": case "join": case "userflags": {
      const previo = chat.usuarios.get(ev.usuario) || {};
      chat.usuarios.set(ev.usuario, { usuario: ev.usuario, cuenta: ev.cuenta, flags: ev.flags,
        personaje: ev.personaje || (ev.tipo === "userflags" ? previo.personaje : null) });
      pintarUsuarios();
      break;
    }
    case "leave":
      if (chat.susurrando && chat.susurrando.toLowerCase() === (ev.cuenta || ev.usuario || "").replace(/^\*/, "").toLowerCase()) soltarSusurro();
      chat.usuarios.delete(ev.usuario);
      pintarUsuarios();
      break;
    case "talk":
      agregarMensaje("", [["msj-nombre", `<${nombreDe(ev.usuario, ev.cuenta)}> `], ["msj-texto", ev.texto]]);
      break;
    case "whisper":
      agregarMensaje("msj-susurro", [["msj-nombre", `De <${nombreDe(ev.usuario, ev.cuenta)}>: `], ["msj-texto", ev.texto]]);
      break;
    case "whispersent":
      agregarMensaje("msj-susurro", [["msj-nombre", `Para <${nombreDe(ev.usuario, ev.cuenta)}>: `], ["msj-texto", ev.texto]]);
      break;
    case "emote":
      agregarMensaje("msj-emote", [["msj-nombre", `<${nombreDe(ev.usuario, ev.cuenta)} `], ["msj-texto", `${ev.texto}>`]]);
      break;
    case "info": if (!respuestaOculta(ev)) agregarMensaje("msj-info", [[null, ev.texto]], true); break;
    case "error": if (!respuestaOculta(ev)) agregarMensaje("msj-error", [[null, ev.texto]], true); break;
    case "broadcast": agregarMensaje("msj-error", [[null, ev.texto]], true); break;
    case "channelfull": agregarMensaje("msj-error", [[null, `El canal ${ev.texto} está lleno.`]]); break;
    case "channeldoesnotexist": agregarMensaje("msj-error", [[null, `El canal ${ev.texto} no existe.`]]); break;
    case "channelrestricted": agregarMensaje("msj-error", [[null, `El canal ${ev.texto} es restringido.`]]); break;
  }
}

// Lo último que se escribió: flecha arriba trae el anterior, flecha abajo vuelve hacia lo que se estaba escribiendo.
const escrito = { lista: [], pos: 0, borrador: "" };
$("sala-texto").addEventListener("keydown", (e) => {
  if (e.key !== "ArrowUp" && e.key !== "ArrowDown") return;
  const t = $("sala-texto"), h = escrito;
  if (!h.lista.length) return;
  e.preventDefault();
  if (h.pos === h.lista.length) h.borrador = t.value;
  h.pos = e.key === "ArrowUp" ? Math.max(0, h.pos - 1) : Math.min(h.lista.length, h.pos + 1);
  t.value = h.pos === h.lista.length ? h.borrador : h.lista[h.pos];
  t.setSelectionRange(t.value.length, t.value.length);
});

$("sala-escribir").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  const t = $("sala-texto").value.trim();
  if (!t) return;
  if (chat.privado) { $("sala-texto").value = ""; enviarPrivado(t); return; }   // conversación de mensajes (mensajes.js)
  if (chat.susurrando && t === prefijoSusurro(chat.susurrando).trim()) return;   // solo el prefijo: nada que mandar
  $("sala-texto").value = chat.susurrando ? prefijoSusurro(chat.susurrando) : "";
  if (escrito.lista[escrito.lista.length - 1] !== t) escrito.lista.push(t);
  if (escrito.lista.length > 50) escrito.lista.shift();
  escrito.pos = escrito.lista.length;
  escrito.borrador = "";
  try {
    await api.ChatEnviar(t);
    anotarComandoIgnorar(t);
    if (!t.startsWith("/")) {
      const yo = chat.estado ? chat.estado.usuario : "";
      agregarMensaje("", [["msj-nombre", `<${yo}> `], ["msj-texto", t]]);
    }
  } catch (err) { agregarMensaje("msj-error", [[null, String(err)]]); }
});
$("sala-canal").addEventListener("click", () => {
  $("sala-canal").hidden = true; $("sala-unirse").hidden = false; $("sala-unirse-nombre").focus();
});
$("sala-unirse").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  const n = $("sala-unirse-nombre").value.trim();
  $("sala-unirse").hidden = true; $("sala-canal").hidden = false; $("sala-unirse-nombre").value = "";
  if (n) try { await api.ChatCanal(n); } catch (err) { agregarMensaje("msj-error", [[null, String(err)]]); }
});
$("sala-unirse-nombre").addEventListener("keydown", (e) => {
  if (e.key === "Escape") { $("sala-unirse").hidden = true; $("sala-canal").hidden = false; }
});

escuchar("chat", alEventoChat);
escuchar("chat-estado", (e) => {
  if (e && !e.conectado) chat.reignorar = null;   // conexión nueva: hay que volver a mandar los ignorados
  if (e && !e.conectado && !e.pausado) { chat.usuarios.clear(); pintarUsuarios(); vozChatCerrado(); mensajesCerrados(); }
  pintarChat(e);
  if (e && (e.conectado || e.pausado)) mensajesAbiertos();   // mensajes.js
});
api.ChatEstado().then(pintarChat).catch(() => {});
if (!enWails && /chat/.test(location.search)) {
  abrirChat();
  if (/sesion/.test(location.search)) entrarChat("launchertest1", "x");
}

// ---------- datos de ejemplo (solo fuera de Wails) ----------
function simulacion() {
  const cfg = { carpeta: "E:\\DIABLO\\DIABLO II - JUGADORES", ventanas: 1, demora: 3, ventana: true, sonido: true, idioma: 1, escala: 100,
    vozModo: 0, vozTecla: 5, vozTeclaNombre: "Mouse 4", vozSensibilidad: 50 };
  const espera = (ms) => new Promise((r) => setTimeout(r, ms));
  let simPendiente = true;
  return {
    Estado: async () => (simPendiente
      ? { version: "1.0.4", config: { ...cfg }, juegoOk: true, pendiente: true, accion: "actualizar", aviso: "pendiente", mensaje: "Hay una actualización de OmbuServer." }
      : { version: "1.0.4", config: { ...cfg }, juegoOk: true, pendiente: false, accion: "jugar", aviso: "ok", mensaje: "El juego está al día." }),
    Actualizar: async () => {
      for (let i = 0; i <= 100; i += 20) { alProgreso({ texto: "Bajando actualización…", porcentaje: i }); await espera(150); }
      simPendiente = false;
      return false;
    },
    ElegirCarpeta: async () => ({ version: "1.0.4", config: { ...cfg }, juegoOk: true, aviso: "ok", mensaje: "El juego está al día." }),
    GuardarOpciones: async (v, d, w, s, i, e, vi) => Object.assign(cfg, { ventanas: v, demora: d, ventana: w, sonido: s, idioma: i, escala: e, vista: vi }),
    GuardarVoz: async (m, t, n, s) => Object.assign(cfg, { vozModo: m, vozTecla: t, vozTeclaNombre: n, vozSensibilidad: s }),
    VozPermiso: async () => { throw new Error("la voz solo anda en el launcher"); },
    VozSilenciar: async () => {},
    VozTeclaHablar: () => {},
    Noticias: async () => [
      { title: "Bienvenido a OmbuServer", date: "12 oct 2026", text: "Diablo II: Lord of Destruction 1.14b con el Mundo 6: Dún Fiacla, escenarios nuevos y misiones con voces. Tocá JUGAR y el launcher se encarga del resto." },
      { title: "Llegó el Mundo 6", date: "11 oct 2026", text: "Después de Baal, Qual-Kehk tiene algo para contarte sobre Sescheron. Hablale en Harrogath para abrir el camino a Dún Fiacla.", link: "https://example.com" },
      { title: "Filtro de ítems", date: "10 oct 2026", text: "Con las teclas 5 a 0 y G ocultás pociones, oro y los ítems que no te interesan. Todo queda guardado para la próxima." },
    ],
    AbrirEnlace: () => {},
    ChatEstado: async () => ({ conectado: false, usuario: "", recordado: "launchertest1" }),
    ChatSalir: async () => emitir("chat-estado", { conectado: false, recordado: "launchertest1" }),
    UsarIdioma: async () => {},
    ChatEnviar: async (t) => {   // como PvPGN: /squelch y /unsquelch cambian el flag 0x20 y responden
      const m = /^\/(un)?squelch \*(\S+)/.exec(t);
      const u = m && [...chat.usuarios.values()].find((x) => x.cuenta.toLowerCase() === m[2].toLowerCase());
      if (!m) return;
      emitir("chat", { tipo: "info", texto: m[1] ? "Ya no se le ignora más." : `${m[2]} esta siendo ignorado.` });
      if (u) emitir("chat", { ...u, tipo: "userflags", flags: m[1] ? u.flags & ~0x20 : u.flags | 0x20 });
    },
    ChatCanal: async (n) => emitir("chat", { tipo: "channel", texto: n }),
    ChatEntrar: async (cuenta) => {
      setTimeout(() => {
        const pj = (nombre, clase, nivel) => ({ nombre, clase, nivel, expansion: true });
        const ev = [
          { tipo: "channel", texto: "OmbuServer" },
          { tipo: "showuser", usuario: cuenta, cuenta, flags: 0x01 },
          { tipo: "showuser", usuario: "Agos", cuenta: "Agos", flags: 0x08, personaje: pj("hechitest", "sor", 99) },
          { tipo: "showuser", usuario: "Lord", cuenta: "Lord", flags: 0x02, personaje: pj("Valdric", "pal", 85) },
          { tipo: "showuser", usuario: "kira", cuenta: "kira", flags: 0x04, personaje: pj("Kiralyn", "ama", 72) },
          { tipo: "showuser", usuario: "Draven", cuenta: "Draven", flags: 0, personaje: pj("Mortis", "nec", 64) },
          { tipo: "showuser", usuario: "bruma", cuenta: "bruma", flags: 0, personaje: pj("Bruma", "dru", 90) },
          { tipo: "showuser", usuario: "Sicaria", cuenta: "Sicaria", flags: 0, personaje: pj("Nyx", "ass", 58) },
          { tipo: "error", texto: `Hola ${cuenta}, bienvenido a OmbuServer!` },
          { tipo: "info", texto: "En este servidor hay 7 cuentas registradas." },
          { tipo: "info", texto: "Ahora hay 7 usuarios conectados, 2 partidas y 4 salas de chat." },
          { tipo: "info", texto: "¿Problemas con algún comando? Teclee \"/help\"." },
          { tipo: "info", texto: "You have no mail." },
          { tipo: "talk", usuario: "Agos", cuenta: "Agos", texto: "Bienvenidos al Mundo 6, ya está abierto Dún Fiacla." },
          { tipo: "talk", usuario: "kira", cuenta: "kira", texto: "alguien para Baal en infierno?" },
          { tipo: "emote", usuario: "Lord", cuenta: "Lord", texto: "se prepara para la batalla" },
          { tipo: "whisper", usuario: "Draven", cuenta: "Draven", texto: "te cambio la Ber por la Jah" },
          { tipo: "whispersent", usuario: "Draven", cuenta: "Draven", texto: "dale, pasame la partida" },
        ];
        ev.forEach((e) => emitir("chat", e));
      }, 50);
      return { conectado: true, usuario: cuenta, recordado: cuenta };
    },
    AbrirCarpeta: () => {},
    Info: async () => ({ version: "1.0.4", servidor: "OmbuServer", carpetaDatos: "C:\\Users\\Mariano\\AppData\\Local\\OmbuServer" }),
    Jugar: async () => {
      for (let i = 0; i <= 100; i += 10) { alProgreso({ texto: "Bajando actualización…", porcentaje: i }); await espera(120); }
      alProgreso({ texto: "Abriendo Diablo II…", porcentaje: -1 });
      await espera(600);
    },
  };
}
