"use strict";

// Mensajes entre jugadores por los TRADES (2026-10-14). Los guarda el VPS web por cuenta (lado Go: mensajes.go) y
// se usan con la cuenta del chat conectada:
//   - en TRADES, tocar un ítem abre el modal para escribirle al dueño (la cuenta que lo dejó en la página TRADE);
//   - en CHAT, la columna de la derecha lista las conversaciones (las nuevas arriba, con los sin leer); tocar una la
//     abre en el lugar del canal, con los mismos renglones, colores y cuadro de escribir del chat. Queda guardada
//     aunque el otro no esté conectado: la lee cuando entra.
//   - cada mensaje (y cada conversación entera) se puede borrar; se borra para uno, el otro conserva su copia.
// Lo que escribe la gente y los nombres van con translate="no".

// confirmar: la conversación que espera el segundo toque para borrarse (sobrevive a los refrescos de la lista)
const buzon = { lista: [], firma: "", cuenta: "", cargando: false, cargado: 0, error: "", borrador: "", imagenes: "", confirmar: "" };
const MENSAJES_MS = 5000;     // la conversación abierta se trae cada 5 s; la columna cada 15 s en CHAT y cada 60 s afuera

const cuentaConectada = () => { const st = chat.estado || {}; return !!((st.conectado || st.pausado) && st.usuario); };
const yoCuenta = () => (chat.estado && chat.estado.usuario) || "";

if (api.TradesImagenes) api.TradesImagenes().then((u) => { buzon.imagenes = u; }).catch(() => {});

// ---------- fechas ----------
const LOCAL = () => (idioma === "en" ? "en-US" : "es-AR");
function horaDe(seg) {
  return new Date(seg * 1000).toLocaleTimeString(LOCAL(), { hour: "2-digit", minute: "2-digit", hourCycle: idioma === "en" ? "h12" : "h23" });
}
function diaDe(seg) {
  const d = new Date(seg * 1000), hoy = new Date();
  const ayer = new Date(hoy.getFullYear(), hoy.getMonth(), hoy.getDate() - 1);
  if (d.toDateString() === hoy.toDateString()) return "Hoy";
  if (d.toDateString() === ayer.toDateString()) return "Ayer";
  return d.toLocaleDateString(LOCAL(), { day: "numeric", month: "short", year: d.getFullYear() === hoy.getFullYear() ? undefined : "numeric" });
}
function cuandoCorto(seg) {
  const d = diaDe(seg);
  return d === "Hoy" ? horaDe(seg) : d;
}

// ---------- columna de mensajes (CHAT) ----------
function marcarSinLeer(n) {
  const g = $("menu-mensajes");
  g.hidden = !n;
  g.textContent = n > 99 ? "99+" : String(n || "");
  $("buzon-cuantos").hidden = !n;
  $("buzon-cuantos").textContent = g.textContent;
}

async function cargarBuzon() {
  if (!cuentaConectada() || buzon.cargando) return;
  buzon.cargando = true;
  const cuenta = yoCuenta();
  try {
    const texto = await api.MensajesConversaciones();
    if (cuenta !== yoCuenta()) return;
    const r = JSON.parse(texto);
    buzon.cargado = Date.now();
    if (texto !== buzon.firma || buzon.error) {
      buzon.firma = texto;
      buzon.error = "";
      buzon.lista = r.conversaciones || [];
      pintarBuzon();
    }
    marcarSinLeer(r.sinLeer || 0);
  } catch (err) {
    buzon.error = String((err && err.message) || err);
    buzon.firma = "";
    pintarBuzon();
  } finally {
    buzon.cargando = false;
  }
}

function pintarBuzon() {
  const cont = $("buzon-lista");
  const scroll = cont.scrollTop;
  cont.textContent = "";
  if (buzon.error) {
    cont.append(el("p", "buzon-vacio buzon-error", buzon.error));
    return;
  }
  if (!buzon.lista.length) {
    cont.append(el("p", "buzon-vacio", buzon.cargado ? "Sin mensajes todavía. Tocá un ítem en TRADES para escribirle al dueño." : "Cargando…"));
    return;
  }
  for (const c of buzon.lista) {
    const fila = el("div", "buzon-item");
    fila.classList.toggle("activo", !!chat.privado && chat.privado.con === c.con);
    fila.classList.toggle("sin-leer", c.sinLeer > 0);
    const arriba = el("div", "buzon-arriba");
    const punto = el("span", c.enLinea ? "trades-punto en-linea" : "trades-punto");
    punto.title = c.enLinea ? "En línea" : "Desconectado";
    const nom = el("span", "buzon-nombre", c.nombre);
    nom.translate = false;
    nom.style.color = colorDueno(c.con);
    arriba.append(punto, nom, el("span", "buzon-cuando", c.ultimo ? cuandoCorto(c.ultimo.fecha) : ""));
    const abajo = el("div", "buzon-abajo");
    const vista = el("span", "buzon-vista");
    if (c.ultimo) {
      if (c.ultimo.mio) vista.append(el("span", "buzon-vos", "Vos: "));
      const t = el("span", "", c.ultimo.texto.replace(/ÿc./g, "").replace(/\s+/g, " "));
      t.translate = false;
      vista.append(t);
    }
    abajo.append(vista);
    if (c.sinLeer) abajo.append(el("span", "buzon-nuevos", String(c.sinLeer)));
    // borrar la conversación: el primer toque pregunta, el segundo borra (se arrepiente solo a los 3 s)
    const borrar = el("button", "buzon-borrar");
    borrar.type = "button";
    if (buzon.confirmar === c.con) {
      fila.classList.add("confirmar");
      borrar.textContent = "¿Borrar?";
      borrar.title = "Tocá de nuevo para borrar la conversación";
    } else {
      borrar.title = "Borrar la conversación";
      borrar.innerHTML = '<svg viewBox="0 0 12 12"><path d="M2.5 2.5l7 7M9.5 2.5l-7 7" /></svg>';
    }
    borrar.addEventListener("click", (e) => { e.stopPropagation(); borrarConversacion(c); });
    fila.append(arriba, abajo, borrar);
    fila.addEventListener("click", () => abrirPrivado(c.con, c.nombre));
    cont.append(fila);
  }
  cont.scrollTop = scroll;
}

let confirmarEspera = null;
async function borrarConversacion(c) {
  clearTimeout(confirmarEspera);
  if (buzon.confirmar !== c.con) {
    buzon.confirmar = c.con;
    pintarBuzon();
    confirmarEspera = setTimeout(() => { buzon.confirmar = ""; pintarBuzon(); }, 3000);
    return;
  }
  buzon.confirmar = "";
  try {
    await api.MensajesBorrar(0, c.con);
    if (chat.privado && chat.privado.con === c.con) cerrarPrivado();
    buzon.lista = buzon.lista.filter((x) => x !== c);
    pintarBuzon();
    cargarBuzon();
  } catch (err) {
    buzon.error = String((err && err.message) || err);
    pintarBuzon();
  }
}

// ---------- conversación privada (en el lugar del canal) ----------
function modoPrivado(si) {
  $("chat-sala").classList.toggle("en-privado", si);
  $("sala-mensajes").hidden = si;
  $("priv-mensajes").hidden = !si;
  $("priv-volver").hidden = !si;
  $("priv-titulo").hidden = !si;
  $("sala-canal").hidden = si;
  $("sala-unirse").hidden = true;
  $("sala-aviso").hidden = si;
}

function pintarCabeceraPrivado(r) {
  const p = chat.privado;
  if (!p) return;
  if (r) { p.nombre = r.nombre || p.nombre; p.enLinea = !!r.enLinea; }
  $("priv-nombre").textContent = p.nombre;
  $("priv-nombre").style.color = colorDueno(p.con);
  $("priv-punto").className = p.enLinea ? "trades-punto en-linea" : "trades-punto";
  $("priv-punto").title = p.enLinea ? "En línea" : "Desconectado";
  $("sala-texto").placeholder = `Mensaje para ${p.nombre}…`;
}

async function abrirPrivado(con, nombre) {
  con = String(con || "").replace(/^\*/, "").toLowerCase();
  if (!con) return;
  if (chat.privado && chat.privado.con === con) { $("sala-texto").focus(); return; }
  if (!chat.privado) buzon.borrador = $("sala-texto").value;   // lo que se estaba escribiendo en el canal
  chat.privado = { con, nombre: nombre || con, enLinea: false, ultimo: 0, dia: "", ids: new Set(), trayendo: false };
  $("priv-mensajes").textContent = "";
  $("sala-texto").value = "";
  modoPrivado(true);
  pintarCabeceraPrivado();
  pintarBuzon();
  $("sala-texto").focus();
  await traerPrivado();
  $("priv-mensajes").scrollTop = $("priv-mensajes").scrollHeight;
}

function cerrarPrivado() {
  if (!chat.privado) return;
  chat.privado = null;
  modoPrivado(false);
  $("sala-texto").value = buzon.borrador;
  buzon.borrador = "";
  $("sala-texto").placeholder = chat.susurrando ? `Susurrando a ${nombreDe(chat.susurrando, chat.susurrando)}…` : "Escribí un mensaje…";
  pintarBuzon();
  const caja = $("sala-mensajes");
  caja.scrollTop = caja.scrollHeight;
}

async function traerPrivado() {
  const p = chat.privado;
  if (!p || p.trayendo) return;
  p.trayendo = true;
  try {
    const r = JSON.parse(await api.MensajesConversacion(p.con, p.ultimo));
    if (chat.privado !== p) return;
    pintarCabeceraPrivado(r);
    let nuevosDelOtro = false;
    for (const m of r.mensajes || []) {
      if (!m.mio && !m.leido) nuevosDelOtro = true;
      renglonPrivado(m);
    }
    if (nuevosDelOtro) cargarBuzon();   // ya quedaron leídos: baja el contador
  } catch (err) {
    if (chat.privado === p) agregarMensaje("msj-error", [[null, String((err && err.message) || err)]], false, $("priv-mensajes"));
  } finally {
    p.trayendo = false;
  }
}

function urlImagen(clave) {
  const base = buzon.imagenes || trades.imagenes;
  return clave && base ? `${base}${clave}.png` : "";
}

// Un mensaje con el formato del chat: hora, <nombre> y el texto; si habla de un ítem, el ítem con su color.
function renglonPrivado(m) {
  const p = chat.privado;
  if (!p || p.ids.has(m.id)) return;
  p.ids.add(m.id);
  p.ultimo = Math.max(p.ultimo, m.id);
  const caja = $("priv-mensajes");
  const dia = diaDe(m.fecha);
  if (dia !== p.dia) {
    p.dia = dia;
    agregarMensaje("msj-canal priv-dia", [[null, "— "], [null, dia], [null, " —"]], false, caja);
  }
  const quien = m.mio ? yoCuenta() : p.nombre;
  const r = agregarMensaje(m.mio ? "priv-mio" : "", [["msj-hora", horaDe(m.fecha) + " "], ["msj-nombre", `<${quien}> `], ["msj-texto", m.texto]], false, caja);
  r.querySelector(".msj-nombre").translate = false;
  if (!m.mio && !m.leido) r.classList.add("priv-nuevo");
  if (m.item && m.item.n) {
    const it = el("span", "priv-item");
    it.translate = false;
    const src = urlImagen(m.item.img);
    if (src) {
      const img = el("img");
      img.alt = "";
      img.src = src;
      img.onerror = () => img.remove();
      it.append(img);
    }
    const n = el("span", "", `[${m.item.n}]`);
    if (m.item.c) n.style.color = m.item.c;
    it.append(n);
    r.querySelector(".msj-texto").before(it, document.createTextNode(" "));
  }
  const borrar = el("button", "msj-borrar");
  borrar.type = "button";
  borrar.title = "Borrar este mensaje";
  borrar.innerHTML = '<svg viewBox="0 0 12 12"><path d="M2.5 2.5l7 7M9.5 2.5l-7 7" /></svg>';
  borrar.addEventListener("click", async () => {
    try {
      await api.MensajesBorrar(m.id, "");
      r.remove();
      cargarBuzon();
    } catch (err) { agregarMensaje("msj-error", [[null, String((err && err.message) || err)]], false, caja); }
  });
  r.append(borrar);
}

// Lo llama el cuadro de escribir del chat (app.js) con una conversación abierta.
async function enviarPrivado(t) {
  const p = chat.privado;
  if (!p) return;
  try {
    const r = JSON.parse(await api.MensajesEnviar(p.con, t, ""));
    if (chat.privado === p) {
      renglonPrivado(r.mensaje);
      const caja = $("priv-mensajes");
      caja.scrollTop = caja.scrollHeight;
    }
    cargarBuzon();
  } catch (err) {
    if (chat.privado === p) {
      agregarMensaje("msj-error", [[null, String((err && err.message) || err)]], false, $("priv-mensajes"));
      if (!$("sala-texto").value) $("sala-texto").value = t;   // que no se pierda lo escrito
    }
  }
}

$("priv-volver").addEventListener("click", cerrarPrivado);
$("sala-texto").addEventListener("keydown", (e) => { if (e.key === "Escape" && chat.privado) cerrarPrivado(); });

// Sesión del chat (app.js): al entrar se cargan los mensajes de esa cuenta; al salir no queda nada a la vista.
function mensajesAbiertos() {
  if (buzon.cuenta.toLowerCase() !== yoCuenta().toLowerCase()) {
    mensajesCerrados();
    buzon.cuenta = yoCuenta();
  } else if (Date.now() - buzon.cargado < 5000) return;
  cargarBuzon();
}

function mensajesCerrados() {
  cerrarPrivado();
  buzon.lista = [];
  buzon.firma = "";
  buzon.cuenta = "";
  buzon.cargado = 0;
  buzon.error = "";
  marcarSinLeer(0);
  pintarBuzon();
}

let mensajesVueltas = 0;
setInterval(() => {
  if (!cuentaConectada() || document.hidden) return;
  const enChat = !$("chat").hidden;
  mensajesVueltas++;
  if (enChat && chat.privado) traerPrivado();
  if (!buzon.cargado || mensajesVueltas % (enChat ? 3 : 12) === 0) cargarBuzon();
}, MENSAJES_MS);
document.querySelector('.menu-item[data-vista="chat"]').addEventListener("click", () => { if (Date.now() - buzon.cargado > 3000) cargarBuzon(); });
document.addEventListener("idioma", () => { pintarBuzon(); if (chat.privado) pintarCabeceraPrivado(); });

// ---------- modal: mensaje al dueño de un ítem (TRADES) ----------
const modal = { x: null, src: "", enviado: false, enviando: false };

function textoInicial(nombre) {
  return idioma === "en" ? `Hi! I'm interested in your ${nombre}. Would you trade it?` : `¡Hola! Me interesa tu ${nombre}. ¿Lo cambiás?`;
}

function pintarModal() {
  const x = modal.x;
  const cuenta = x.cuenta || "";
  const nota = $("msj-modal-nota"), enviar = $("msj-modal-enviar"), texto = $("msj-modal-texto");
  nota.className = "modal-nota";
  $("msj-modal-cancelar").textContent = "Cancelar";
  enviar.disabled = modal.enviando;
  texto.disabled = false;
  enviar.textContent = modal.enviando ? "Enviando…" : "Enviar";
  if (!cuentaConectada()) {
    nota.textContent = "Entrá al chat con tu cuenta para mandar y leer mensajes.";
    enviar.textContent = "Ir al chat";
    texto.disabled = true;
  } else if (!cuenta) {
    nota.textContent = "No se sabe la cuenta del dueño de este ítem.";
    enviar.disabled = texto.disabled = true;
  } else if (cuenta.toLowerCase() === yoCuenta().toLowerCase()) {
    nota.textContent = "Este ítem es tuyo.";
    enviar.disabled = texto.disabled = true;
  } else if (modal.enviado) {
    nota.textContent = "Mensaje enviado. La conversación queda en CHAT, en la columna de mensajes.";
    nota.classList.add("ok");
    enviar.textContent = "Ver conversación";
    $("msj-modal-cancelar").textContent = "Cerrar";
    texto.disabled = true;
  } else {
    nota.textContent = "";
  }
  $("msj-modal-largo").textContent = `${texto.value.length} / 500`;
}

// Lo llama trades.js al tocar un ítem (x: el ítem de la búsqueda, src: su imagen).
function abrirMensajeItem(x, src) {
  Object.assign(modal, { x, src, enviado: false, enviando: false });
  const it = x.item;
  const marco = $("msj-modal-img");
  marco.textContent = "";
  marco.style.setProperty("--brillo", colorNombre(it));
  if (src) {
    const img = el("img");
    img.alt = "";
    img.src = src;
    img.onerror = () => img.remove();
    marco.append(img);
  }
  $("msj-modal-nombre").textContent = x.nombre;
  $("msj-modal-nombre").style.color = colorNombre(it);
  $("msj-modal-dueno").textContent = dueno(x);
  $("msj-modal-dueno").style.color = colorDueno(x.cuenta || x.pj);
  $("msj-modal-pj").textContent = x.cuenta && x.pj ? ` · ${x.pj}` : "";
  $("msj-modal-punto").className = x.enLinea ? "trades-punto en-linea" : "trades-punto";
  $("msj-modal-texto").value = textoInicial(x.nombre);
  pintarModal();
  $("msj-modal").hidden = false;
  const t = $("msj-modal-texto");
  if (!t.disabled) { t.focus(); t.setSelectionRange(t.value.length, t.value.length); }
  else $("msj-modal-enviar").focus();
}

function cerrarModal() { $("msj-modal").hidden = true; modal.x = null; }

$("msj-modal-form").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  const x = modal.x;
  if (!x || modal.enviando) return;
  if (!cuentaConectada()) { cerrarModal(); abrirChat(); return; }
  if (modal.enviado) { cerrarModal(); abrirChat(); abrirPrivado(x.cuenta, x.cuenta); return; }
  const texto = $("msj-modal-texto").value.trim();
  if (!texto) { $("msj-modal-texto").focus(); return; }
  const src = modal.src || "";
  const clave = src.startsWith(trades.imagenes) ? src.slice(trades.imagenes.length).replace(/\.png$/, "") : "";
  modal.enviando = true;
  pintarModal();
  try {
    await api.MensajesEnviar(x.cuenta, texto, JSON.stringify({ n: x.nombre, c: colorNombre(x.item), img: clave }));
    if (modal.x !== x) return;
    modal.enviado = true;
    cargarBuzon();
  } catch (err) {
    if (modal.x !== x) return;
    modal.enviando = false;
    pintarModal();
    $("msj-modal-nota").textContent = String((err && err.message) || err);
    $("msj-modal-nota").classList.add("error");
    return;
  }
  modal.enviando = false;
  pintarModal();
  $("msj-modal-enviar").focus();
});
$("msj-modal-texto").addEventListener("input", () => { $("msj-modal-largo").textContent = `${$("msj-modal-texto").value.length} / 500`; });
// Enter manda (como el chat); Shift+Enter hace un renglón nuevo.
$("msj-modal-texto").addEventListener("keydown", (e) => {
  if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); $("msj-modal-form").requestSubmit(); }
});
$("msj-modal-cerrar").addEventListener("click", cerrarModal);
$("msj-modal-cancelar").addEventListener("click", cerrarModal);
$("msj-modal").addEventListener("mousedown", (e) => { if (e.target === $("msj-modal")) cerrarModal(); });
document.addEventListener("keydown", (e) => { if (e.key === "Escape" && !$("msj-modal").hidden) cerrarModal(); });

pintarBuzon();

// ---------- datos de ejemplo (solo fuera de Wails, para revisar el diseño en un navegador) ----------
if (!enWails) {
  const ahora = Math.floor(Date.now() / 1000);
  const sim = {
    agos: { nombre: "Agos", enLinea: true, mensajes: [
      { id: 1, mio: true, texto: "¡Hola! Me interesa tu Corona de Harlequín. ¿Lo cambiás?", item: { n: "Corona de Harlequín", c: "#c7b377", img: "" }, fecha: ahora - 90000, leido: true },
      { id: 2, mio: false, texto: "Dale, ¿qué me ofrecés? Busco runas altas.", fecha: ahora - 86000, leido: true },
      { id: 5, mio: true, texto: "Te doy una Ist y una Mal.", fecha: ahora - 3000, leido: true },
      { id: 6, mio: false, texto: "Hecho. Entrá a la partida Trade-1, clave 1.", fecha: ahora - 600, leido: false },
      { id: 7, mio: false, texto: "Estoy en el acto 1.", fecha: ahora - 540, leido: false }] },
    lucho: { nombre: "lucho", enLinea: false, mensajes: [
      { id: 3, mio: false, texto: "¿Seguís teniendo el Enigma?", fecha: ahora - 200000, leido: true },
      { id: 4, mio: true, texto: "Sí, todavía lo tengo en el trade.", fecha: ahora - 190000, leido: true }] },
    arena2: { nombre: "arena2", enLinea: false, mensajes: [
      { id: 8, mio: true, texto: "¿Cuánto pedís por la Ojo del Halcón?", item: { n: "Ojo del Halcón", c: "#c7b377", img: "" }, fecha: ahora - 400000, leido: true }] },
  };
  let simId = 100;
  Object.assign(api, {
    MensajesConversaciones: async () => {
      const lista = Object.entries(sim).filter(([, c]) => c.mensajes.length)
        .map(([con, c]) => ({ con, nombre: c.nombre, enLinea: c.enLinea, sinLeer: c.mensajes.filter((m) => !m.mio && !m.leido).length, ultimo: c.mensajes[c.mensajes.length - 1] }))
        .sort((a, b) => b.ultimo.id - a.ultimo.id);
      return JSON.stringify({ ok: true, cuenta: yoCuenta(), sinLeer: lista.reduce((n, c) => n + c.sinLeer, 0), conversaciones: lista });
    },
    MensajesConversacion: async (con, desde) => {
      const c = sim[con] || { nombre: con, mensajes: [] };
      const r = JSON.stringify({ ok: true, con, nombre: c.nombre, enLinea: c.enLinea, mensajes: c.mensajes.filter((m) => m.id > desde) });
      c.mensajes.forEach((m) => { if (!m.mio) m.leido = true; });
      return r;
    },
    MensajesEnviar: async (para, texto, item) => {
      const c = sim[para.toLowerCase()] = sim[para.toLowerCase()] || { nombre: para, enLinea: false, mensajes: [] };
      const m = { id: ++simId, mio: true, texto, item: item ? JSON.parse(item) : null, fecha: Math.floor(Date.now() / 1000), leido: false };
      c.mensajes.push(m);
      return JSON.stringify({ ok: true, mensaje: m });
    },
    MensajesBorrar: async (id, con) => {
      for (const [k, c] of Object.entries(sim)) {
        if (id) c.mensajes = c.mensajes.filter((m) => m.id !== id);
        else if (k === con) c.mensajes = [];
      }
    },
  });
}
