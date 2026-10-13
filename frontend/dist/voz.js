"use strict";
// Voz del chat: una sala de voz (LiveKit, en el VPS web) por canal privado. El permiso lo da el servidor
// (voz-token.py): solo canales privados, solo quien está en el canal, y silenciar para todos solo el operador del
// canal (el que lo creó) o un admin. La tecla para hablar la lee el launcher en Go (voz.go), así anda con el juego
// en primer plano. Usa: $, api, chat, escuchar, estado (app.js) y LivekitClient (vendor/livekit-client.umd.js).

const LK = window.LivekitClient;
// Canales fijos del servidor (channel.conf): ahí no hay voz. El servidor lo controla igual; esto es para no ofrecerla.
const CANALES_FIJOS = ["the void", "ombuserver", "backstage", "moderated support", "chat", "diablo ii"];
const MANTENER_VOZ_MS = 400;      // detección de voz: sigue al aire un rato después de la última palabra
const SOLTAR_TECLA_MS = 180;      // tecla para hablar: no cortar la última sílaba

const voz = {
  room: null, sala: "", identidad: "", operador: false,
  mic: null,                       // LocalAudioTrack del micrófono
  alAire: false,                   // se está mandando audio ahora
  silenciadoPorMi: false,          // botón "Silenciar micrófono"
  silenciadoPorOperador: false,
  cargando: false,                 // entrando a la voz o reconectando (el botón gira)
  sinMic: false,                   // no hay micrófono (o Windows no lo deja usar): solo escucha
  hablando: new Set(),             // identidades que están hablando
  noEscucho: new Set(cargarNoEscucho()), // silenciados solo para mí (se recuerdan)
  audio: null,                     // { stream, ctx, nodo, pista }: micrófono -> compuerta -> LiveKit
  ultimoDb: -100,                  // nivel del micrófono que informa la compuerta
  analisis: null,                  // medidor de "Probar micrófono" fuera de la voz
};

function cargarNoEscucho() {
  try { return JSON.parse(localStorage.getItem("voz-no-escucho") || "[]"); } catch (_) { return []; }
}
function guardarNoEscucho() {
  try { localStorage.setItem("voz-no-escucho", JSON.stringify([...voz.noEscucho])); } catch (_) { /* sin almacenamiento */ }
}

const cfgVoz = () => (estado && estado.config) || {};
const esPrivado = (canal) => !!canal && !CANALES_FIJOS.includes(canal.toLowerCase());
const cuentaDe = (u) => (u.cuenta || u.usuario || "").replace(/^\*/, "").toLowerCase();

// ---------- botón de micrófono (al final del renglón de escribir) y su panel ----------
function pintarBarraVoz() {
  const b = $("voz-mic");
  b.hidden = !esPrivado(chat.canal) && !voz.room;
  const dentro = !!voz.room;
  const callado = dentro && (voz.sinMic || voz.silenciadoPorMi || voz.silenciadoPorOperador);
  b.classList.toggle("dentro", dentro);
  b.classList.toggle("al-aire", dentro && voz.alAire);
  b.classList.toggle("callado", callado && !voz.cargando);
  b.classList.toggle("cargando", voz.cargando);
  const n = dentro ? (voz.room.numParticipants || (voz.room.remoteParticipants.size + 1)) : 0;
  $("voz-cuantos").textContent = n > 1 ? n : "";
  const c = cfgVoz();
  let t;
  if (voz.cargando) t = dentro ? "Reconectando la voz…" : "Entrando a la voz…";
  else if (!dentro) t = "Entrar a la voz del canal";
  else if (voz.sinMic) t = "Sin micrófono: solo escuchás";
  else if (voz.silenciadoPorOperador) t = "El operador del canal te sacó la voz";
  else if (voz.silenciadoPorMi) t = "Micrófono silenciado";
  else if (c.vozModo === 1) t = "Detección de voz: hablá y te escuchan";
  else t = `Mantené ${c.vozTeclaNombre || "la tecla"} para hablar`;
  b.title = dentro ? `${t} · ${n} en la voz` : t;
  $("voz-estado").textContent = dentro ? `${t} · ${n} en la voz` : "";
  $("voz-silenciar").hidden = !dentro || voz.sinMic || voz.silenciadoPorOperador;
  $("voz-silenciar").textContent = voz.silenciadoPorMi ? "Activar micrófono" : "Silenciar micrófono";
  $("voz-reintentar").hidden = !dentro || !voz.sinMic;
  if (!dentro) $("voz-panel").hidden = true;
}

function errorVoz(t) {
  if (t) agregarMensaje("msj-error", [[null, t]]);
}

// Errores del micrófono, en castellano y con qué hacer.
function textoErrorMic(err) {
  const n = err && err.name;
  if (n === "NotFoundError" || n === "OverconstrainedError") return "No se encontró un micrófono: entraste a la voz solo para escuchar.";
  if (n === "NotAllowedError" || n === "SecurityError") return "Windows no deja usar el micrófono: Configuración de Windows → Privacidad y seguridad → Micrófono → activá \"Permitir que las aplicaciones de escritorio accedan al micrófono\". Mientras tanto, solo escuchás.";
  if (n === "NotReadableError" || n === "AbortError") return "El micrófono está ocupado por otro programa: entraste solo para escuchar.";
  return "No se pudo abrir el micrófono (" + (err && err.message || err) + "): entraste solo para escuchar.";
}

// Marca en la tira de personajes quién está en la voz, quién habla y a quién no se escucha.
function vozMarcarTira() {
  const enVoz = new Set();
  const callados = new Set(voz.noEscucho);
  if (voz.room) {
    enVoz.add(voz.identidad);
    for (const p of voz.room.remoteParticipants.values()) {
      enVoz.add(p.identity);
      if (p.permissions && p.permissions.canPublish === false) callados.add(p.identity);
    }
    if (voz.silenciadoPorOperador || voz.silenciadoPorMi || voz.sinMic) callados.add(voz.identidad);
  }
  document.querySelectorAll("#sala-usuarios .persona").forEach((d) => {
    const id = d.dataset.cuenta;
    d.classList.toggle("en-voz", enVoz.has(id));
    d.classList.toggle("hablando", voz.hablando.has(id) || (id === voz.identidad && voz.alAire));
    d.classList.toggle("callado", enVoz.has(id) && callados.has(id));
  });
}

// ---------- entrar y salir ----------
async function entrarVoz() {
  if (voz.room || !LK) return;
  if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
    errorVoz("El micrófono no está disponible en este equipo.");
    return;
  }
  $("voz-mic").disabled = true;
  voz.cargando = true;
  pintarBarraVoz();
  let room = null;
  try {
    const sala = chat.canal;
    const p = await api.VozPermiso(sala);
    room = new LK.Room({
      adaptiveStream: false, dynacast: false,
      audioCaptureDefaults: { echoCancellation: true, noiseSuppression: true, autoGainControl: true },
    });
    room
      .on(LK.RoomEvent.TrackSubscribed, (track, _pub, part) => {
        if (track.kind !== LK.Track.Kind.Audio) return;
        $("voz-audio").append(track.attach());
        if (voz.noEscucho.has(part.identity)) part.setVolume(0);
      })
      .on(LK.RoomEvent.TrackUnsubscribed, (track) => track.detach().forEach((e) => e.remove()))
      .on(LK.RoomEvent.ActiveSpeakersChanged, (quienes) => {
        voz.hablando = new Set(quienes.filter((q) => q.identity !== voz.identidad).map((q) => q.identity));
        vozMarcarTira();
      })
      .on(LK.RoomEvent.ParticipantConnected, () => { pintarBarraVoz(); vozMarcarTira(); })
      .on(LK.RoomEvent.ParticipantDisconnected, () => { pintarBarraVoz(); vozMarcarTira(); })
      .on(LK.RoomEvent.ParticipantPermissionsChanged, (_antes, part) => {
        if (part === room.localParticipant) alCambiarMiPermiso();
        vozMarcarTira();
      })
      .on(LK.RoomEvent.Reconnecting, () => { voz.cargando = true; pintarBarraVoz(); })
      .on(LK.RoomEvent.Reconnected, () => { voz.cargando = false; pintarBarraVoz(); })
      .on(LK.RoomEvent.Disconnected, () => { if (voz.room === room) limpiarVoz("Se cortó la voz."); });
    await room.connect(p.url, p.token);
    Object.assign(voz, { room, sala, identidad: p.identidad, operador: p.operador,
      silenciadoPorOperador: !!p.silenciado, silenciadoPorMi: false, hablando: new Set() });
    await room.startAudio();
    agregarMensaje("msj-info", [[null, "Entraste a la voz del canal."]]);
    if (!voz.silenciadoPorOperador) await intentarMicrofono();
    pintarBarraVoz();
    vozMarcarTira();
  } catch (err) {
    if (room) room.disconnect();
    limpiarVoz();
    errorVoz("No se pudo entrar a la voz: " + String(err && err.message || err));
  } finally {
    voz.cargando = false;
    $("voz-mic").disabled = false;
    pintarBarraVoz();
  }
}

// ---------- micrófono: una compuerta en el hilo de audio ----------
// El micrófono se publica una sola vez y queda siempre activo; lo que pasa o no pasa lo decide una compuerta que corre
// en el hilo de audio (AudioWorklet). Ahí no la frena el navegador aunque el launcher esté minimizado o en segundo
// plano (los temporizadores sí se frenan), y abre al instante: no se pierde el comienzo de las palabras.
//   modo 1 (detección de voz): abre cuando el nivel pasa el umbral y sigue abierta MANTENER_VOZ_MS;
//   modo 0 (tecla): abre mientras la tecla esté apretada (la lee Go) y SOLTAR_TECLA_MS después de soltarla.
const COMPUERTA = `
class OmbuCompuerta extends AudioWorkletProcessor {
  constructor() {
    super();
    Object.assign(this, { modo: 0, umbral: -45, mantener: 0.4, tecla: false, hasta: 0, silencio: true,
      ganancia: 0, abierta: false, ultimoNivel: 0 });
    this.port.onmessage = (e) => Object.assign(this, e.data);
  }
  process(entradas, salidas) {
    const ent = entradas[0], sal = salidas[0];
    if (!ent || !ent.length) return true;
    const x = ent[0];
    let s = 0;
    for (let i = 0; i < x.length; i++) s += x[i] * x[i];
    const db = 20 * Math.log10(Math.sqrt(s / x.length) + 1e-8);
    const t = currentTime;
    let abrir = false;
    if (!this.silencio) {
      if (this.modo === 1) { if (db > this.umbral) this.hasta = t + this.mantener; abrir = t < this.hasta; }
      else abrir = this.tecla || t < this.hasta;
    }
    const objetivo = abrir ? 1 : 0;
    let g = this.ganancia;
    for (let i = 0; i < x.length; i++) {
      g += (objetivo - g) * 0.01;              // rampa de ~2 ms: sin chasquidos
      for (let c = 0; c < sal.length; c++) sal[c][i] = (ent[c] || x)[i] * g;
    }
    this.ganancia = g;
    if (abrir !== this.abierta) { this.abierta = abrir; this.port.postMessage({ abierta: abrir }); }
    if (t - this.ultimoNivel > 0.05) { this.ultimoNivel = t; this.port.postMessage({ db }); }
    return true;
  }
}
registerProcessor("ombu-compuerta", OmbuCompuerta);
`;
const URL_COMPUERTA = URL.createObjectURL(new Blob([COMPUERTA], { type: "application/javascript" }));
const RESTRICCIONES_MIC = { echoCancellation: true, noiseSuppression: true, autoGainControl: true };

// Abre el micrófono; si no hay (o Windows no lo deja), se queda en la voz solo escuchando.
async function intentarMicrofono() {
  try {
    await abrirMicrofono();
    voz.sinMic = false;
  } catch (err) {
    cerrarMicrofono();
    voz.sinMic = true;
    errorVoz(textoErrorMic(err));
  }
  pintarBarraVoz();
  vozMarcarTira();
}

// La compuerta como procesador de LiveKit (setProcessor): toma la pista del micrófono y entrega la suya, que LiveKit pone
// en la conexión con replaceTrack, sin renegociar.
function crearCompuerta() {
  return {
    name: "ombu-compuerta",
    processedTrack: undefined,
    async init({ track, audioContext: ctx }) {
      if (!ctx.ombuCompuerta) { await ctx.audioWorklet.addModule(URL_COMPUERTA); ctx.ombuCompuerta = true; }
      this.fuente = ctx.createMediaStreamSource(new MediaStream([track]));
      this.nodo = new AudioWorkletNode(ctx, "ombu-compuerta", { outputChannelCount: [1] });
      this.destino = ctx.createMediaStreamDestination();
      this.destino.channelCount = 1;
      this.destino.channelCountMode = "explicit";
      this.fuente.connect(this.nodo);
      this.nodo.connect(this.destino);
      this.nodo.port.onmessage = (e) => {
        if ("abierta" in e.data) { voz.alAire = e.data.abierta; pintarBarraVoz(); vozMarcarTira(); }
        if ("db" in e.data) { voz.ultimoDb = e.data.db; if (prueba.activa) pintarMedidor(e.data.db); }
      };
      this.processedTrack = this.destino.stream.getAudioTracks()[0];
      if (voz.audio) voz.audio.nodo = this.nodo;
      configurarCompuerta();
    },
    async restart(opciones) { await this.destroy(); await this.init(opciones); },
    async destroy() {
      try { this.fuente.disconnect(); this.nodo.disconnect(); } catch (_) { /* ya estaba desarmada */ }
      if (this.processedTrack) this.processedTrack.stop();
    },
  };
}

// El micrófono lo abre y lo publica LiveKit (el camino que anda en el WebView2 de Windows); después se le engancha la
// compuerta. Si la primera publicación falla ("Transceiver not found based on m-line index"), el segundo intento
// anda: se reintenta solo una vez.
async function abrirMicrofono() {
  await cerrarMicrofono();
  const lp = voz.room.localParticipant;
  const ctx = new AudioContext({ latencyHint: "interactive" });
  voz.audio = { ctx, nodo: null, pista: null };
  const publicar = () => lp.setMicrophoneEnabled(true, RESTRICCIONES_MIC);
  try {
    await publicar();
  } catch (err) {
    if (!/transceiver|m-line|createOffer|setLocalDescription/i.test(String(err && err.message))) throw err;
    const quedo = lp.getTrackPublication(LK.Track.Source.Microphone);
    if (quedo && quedo.track) { try { await lp.unpublishTrack(quedo.track, true); } catch (_) { /* no había quedado */ } }
    await new Promise((r) => setTimeout(r, 400));
    await publicar();
  }
  const pub = lp.getTrackPublication(LK.Track.Source.Microphone);
  const pista = pub && pub.track;
  if (!pista) throw new Error("no se pudo abrir el micrófono");
  voz.audio.pista = pista;
  await pista.mute();                       // callada mientras se engancha la compuerta (no sale audio crudo)
  pista.setAudioContext(ctx);
  if (ctx.state !== "running") await ctx.resume();
  await pista.setProcessor(crearCompuerta());
  await pista.unmute();                     // desde acá decide la compuerta
  voz.mic = pista;
  voz.alAire = false;
  prepararModo();
}

function cerrarMicrofono() {
  const a = voz.audio;
  voz.audio = null;
  voz.mic = null;
  voz.alAire = false;
  if (!a) return Promise.resolve();
  if (a.pista) {
    const lp = voz.room && voz.room.localParticipant;
    return Promise.resolve()
      .then(() => a.pista.stopProcessor && a.pista.stopProcessor())
      .then(() => lp && lp.unpublishTrack(a.pista, true))
      .catch(() => { /* ya no estaba */ })
      .finally(() => a.ctx.close());
  }
  return a.ctx.close().catch(() => {});
}

// Le dice a la compuerta cómo tiene que trabajar (modo, umbral, si está silenciado).
function configurarCompuerta() {
  if (!voz.audio || !voz.audio.nodo) return;
  const c = cfgVoz();
  voz.audio.nodo.port.postMessage({
    modo: c.vozModo === 1 ? 1 : 0, umbral: umbralDb(c.vozSensibilidad ?? 50), mantener: MANTENER_VOZ_MS / 1000,
    silencio: voz.silenciadoPorMi || voz.silenciadoPorOperador, tecla: false, hasta: 0,
  });
}

// El operador del canal le sacó (o devolvió) la voz a este launcher.
async function alCambiarMiPermiso() {
  const puede = voz.room.localParticipant.permissions ? voz.room.localParticipant.permissions.canPublish !== false : true;
  voz.silenciadoPorOperador = !puede;
  if (!puede) {
    detenerModo();
    cerrarMicrofono();
  } else if (!voz.mic) {
    await intentarMicrofono();
  }
  pintarBarraVoz();
}

async function salirVoz() {
  const r = voz.room;
  limpiarVoz();
  if (r) await r.disconnect();
}

function limpiarVoz(motivo) {
  detenerModo();
  cerrarMicrofono();
  voz.room = null;
  voz.hablando = new Set();
  voz.sinMic = false;
  voz.cargando = false;
  $("voz-audio").textContent = "";
  errorVoz(motivo || "");
  pintarBarraVoz();
  vozMarcarTira();
}

// ---------- cómo se habla: tecla o detección de voz ----------
function prepararModo() {
  detenerModo();
  configurarCompuerta();
  if (voz.mic && cfgVoz().vozModo !== 1) api.VozTeclaHablar(true);
}

function detenerModo() {
  if (api.VozTeclaHablar) api.VozTeclaHablar(false);
}

escuchar("voz-tecla", (apretada) => {
  if (!voz.audio || !voz.audio.nodo || cfgVoz().vozModo === 1) return;
  const ahora = voz.audio.ctx.currentTime;
  voz.audio.nodo.port.postMessage(apretada ? { tecla: true } : { tecla: false, hasta: ahora + SOLTAR_TECLA_MS / 1000 });
});

// Nivel del micrófono en dB para "Probar micrófono" cuando no se está en la voz.
function iniciarAnalisis(pista) {
  detenerAnalisis();
  const ctx = new AudioContext();
  const fuente = ctx.createMediaStreamSource(new MediaStream([pista]));
  const analizador = ctx.createAnalyser();
  analizador.fftSize = 1024;
  fuente.connect(analizador);
  voz.analisis = { ctx, analizador, datos: new Float32Array(analizador.fftSize) };
}

function detenerAnalisis() {
  const a = voz.analisis;
  if (!a) return;
  voz.analisis = null;
  a.ctx.close();
}

function nivelDb() {
  if (voz.audio) return voz.ultimoDb ?? -100;      // en la voz: lo informa la compuerta
  const a = voz.analisis;
  if (!a) return -100;
  a.analizador.getFloatTimeDomainData(a.datos);
  let s = 0;
  for (const v of a.datos) s += v * v;
  return 20 * Math.log10(Math.sqrt(s / a.datos.length) + 1e-8);
}

// Sensibilidad 0..100 -> umbral de -20 dB (hay que hablar fuerte) a -70 dB (capta todo).
const umbralDb = (sens) => -20 - (sens * 0.5);
const dbAPorcentaje = (db) => Math.max(0, Math.min(100, (db + 80) * 1.25));

// ---------- clic derecho sobre un personaje ----------
function vozMenuPersona(ev, u) {
  ev.preventDefault();
  const id = cuentaDe(u);
  const menu = $("menu-persona");
  menu.textContent = "";
  const titulo = document.createElement("div");
  titulo.className = "titulo";
  titulo.translate = false;   // es el nombre del personaje
  titulo.textContent = (u.personaje && u.personaje.nombre) || u.cuenta;
  menu.append(titulo);
  const opcion = (texto, f, peligro) => {
    const b = document.createElement("button");
    b.textContent = texto;
    if (peligro) b.className = "peligro";
    b.addEventListener("click", () => { cerrarMenuPersona(); f(); });
    menu.append(b);
  };
  const elegido = chat.susurrando && chat.susurrando.toLowerCase() === (u.cuenta || "").toLowerCase();
  opcion(elegido ? "Dejar de susurrarle" : "Susurrar", () => elegirSusurro(u.cuenta));
  const esYo = voz.room ? id === voz.identidad : (chat.estado && (chat.estado.usuario || "").toLowerCase() === id);
  // Ignorar es del chat (app.js): no ver lo que escribe ni sus susurros. Mutear es de la voz: no oírlo.
  if (!esYo) {
    if (u.flags & IGNORADO) opcion("No ignorar", () => ignorar(u.cuenta, false));
    else opcion("Ignorar", () => ignorar(u.cuenta, true));
  }
  // Las opciones de voz, solo para quien está en la sala de voz (desde el juego nadie entra a la voz).
  const enLaVoz = !!(voz.room && voz.room.remoteParticipants.get(id));
  if (!esYo && (enLaVoz || voz.noEscucho.has(id))) {
    if (voz.noEscucho.has(id)) opcion("Desmutear", () => escucharA(id, true));
    else opcion("Mutear", () => escucharA(id, false));
    if (enLaVoz && voz.operador) {
      const part = voz.room.remoteParticipants.get(id);
      const callado = part && part.permissions && part.permissions.canPublish === false;
      if (callado) opcion("Desmutear para todos", () => silenciarParaTodos(id, false));
      else opcion("Mutear para todos", () => silenciarParaTodos(id, true), true);
    }
  }
  menu.hidden = false;
  const w = menu.offsetWidth, h = menu.offsetHeight;
  menu.style.left = Math.min(ev.clientX, innerWidth - w - 4) + "px";
  menu.style.top = Math.min(ev.clientY, innerHeight - h - 4) + "px";
}

function cerrarMenuPersona() { $("menu-persona").hidden = true; }
document.addEventListener("mousedown", (e) => { if (!$("menu-persona").contains(e.target)) cerrarMenuPersona(); });
document.addEventListener("keydown", (e) => { if (e.key === "Escape") cerrarMenuPersona(); });

function escucharA(id, si) {
  if (si) voz.noEscucho.delete(id); else voz.noEscucho.add(id);
  guardarNoEscucho();
  const part = voz.room && voz.room.remoteParticipants.get(id);
  if (part) part.setVolume(si ? 1 : 0);
  vozMarcarTira();
}

async function silenciarParaTodos(id, silenciar) {
  try {
    await api.VozSilenciar(voz.sala, id, silenciar);
    agregarMensaje("msj-info", [[null, silenciar ? `${id} no puede hablar en la voz del canal.` : `${id} puede volver a hablar en la voz.`]]);
  } catch (err) {
    agregarMensaje("msj-error", [[null, String(err && err.message || err)]]);
  }
  vozMarcarTira();
}

// ---------- enganches con el chat (app.js) ----------
function vozCambioCanal(canal) {
  if (voz.room && canal.toLowerCase() !== voz.sala.toLowerCase()) salirVoz();
  pintarBarraVoz();
}

function vozChatCerrado() {
  if (voz.room) salirVoz();
  pintarBarraVoz();
}

$("voz-mic").addEventListener("click", (e) => {
  e.stopPropagation();
  if (!voz.room) { entrarVoz(); return; }
  const panel = $("voz-panel");
  panel.hidden = !panel.hidden;
  if (!panel.hidden) {        // pegado arriba del botón, alineado a su derecha
    const r = $("voz-mic").getBoundingClientRect();
    panel.style.right = Math.max(4, innerWidth - r.right) + "px";
    panel.style.bottom = (innerHeight - r.top + 6) + "px";
  }
});
$("voz-salir").addEventListener("click", () => { $("voz-panel").hidden = true; salirVoz(); agregarMensaje("msj-info", [[null, "Saliste de la voz."]]); });
$("voz-reintentar").addEventListener("click", () => { $("voz-panel").hidden = true; intentarMicrofono(); });
$("voz-silenciar").addEventListener("click", () => {
  voz.silenciadoPorMi = !voz.silenciadoPorMi;
  configurarCompuerta();
  pintarBarraVoz();
  vozMarcarTira();
});
document.addEventListener("mousedown", (e) => {
  if (!$("voz-panel").contains(e.target) && e.target.closest("#voz-mic") === null) $("voz-panel").hidden = true;
});
// Los botones laterales del mouse no tienen que hacer "atrás" en la ventana del launcher.
window.addEventListener("mouseup", (e) => { if (e.button === 3 || e.button === 4) e.preventDefault(); });

// ---------- configuración: sección Voz ----------
const prueba = { activa: false, fin: 0, stream: null, intervalo: 0 };

function vozPintarOpciones() {
  const c = cfgVoz();
  const modo = c.vozModo === 1 ? 1 : 0;
  document.querySelectorAll("#op-voz-modo button").forEach((b) => b.classList.toggle("activo", Number(b.dataset.modo) === modo));
  if (!capturando) $("op-voz-tecla").textContent = c.vozTeclaNombre || "Mouse 4";
  $("campo-voz-tecla").classList.toggle("deshabilitado", modo !== 0);
  $("op-voz-sensibilidad").value = c.vozSensibilidad ?? 50;
  $("voz-medidor-umbral").style.left = dbAPorcentaje(umbralDb(Number($("op-voz-sensibilidad").value))) + "%";
}

async function guardarVoz(cambios) {
  const c = Object.assign({ vozModo: 0, vozTecla: 5, vozTeclaNombre: "Mouse 4", vozSensibilidad: 50 }, cfgVoz(), cambios);
  try {
    const nueva = await api.GuardarVoz(c.vozModo, c.vozTecla, c.vozTeclaNombre, c.vozSensibilidad);
    if (estado) estado.config = nueva;
    const g = $("guardado");
    g.classList.add("visible");
    setTimeout(() => g.classList.remove("visible"), 1500);
  } catch (err) {
    aviso("error", String(err));
  }
  vozPintarOpciones();
  if (voz.room) prepararModo();     // modo, tecla o umbral nuevos, en el momento
  pintarBarraVoz();
}

document.querySelectorAll("#op-voz-modo button").forEach((b) => b.addEventListener("click", () => {
  const m = Number(b.dataset.modo);
  if (m !== cfgVoz().vozModo) guardarVoz({ vozModo: m });
}));

let guardarSensibilidad = 0;
$("op-voz-sensibilidad").addEventListener("input", () => {
  const v = Number($("op-voz-sensibilidad").value);
  if (estado && estado.config) estado.config.vozSensibilidad = v;
  $("voz-medidor-umbral").style.left = dbAPorcentaje(umbralDb(v)) + "%";
  configurarCompuerta();
  clearTimeout(guardarSensibilidad);
  guardarSensibilidad = setTimeout(() => guardarVoz({ vozSensibilidad: v }), 400);
});

// Elegir la tecla: la próxima tecla o botón del mouse (no el izquierdo ni el derecho). Esc cancela.
// keyCode en Chromium para Windows es la tecla virtual de Windows (VK_*), la misma que lee GetAsyncKeyState.
let capturando = false;
const NOMBRES_TECLA = { " ": "Espacio", CapsLock: "Bloq Mayús", Tab: "Tab", Enter: "Enter", Backspace: "Retroceso",
  Insert: "Insert", Delete: "Supr", Home: "Inicio", End: "Fin", PageUp: "Re Pág", PageDown: "Av Pág",
  ArrowUp: "Flecha arriba", ArrowDown: "Flecha abajo", ArrowLeft: "Flecha izquierda", ArrowRight: "Flecha derecha",
  Pause: "Pausa", ScrollLock: "Bloq Despl", ContextMenu: "Menú" };

function terminarCaptura() {
  capturando = false;
  $("op-voz-tecla").classList.remove("esperando");
  window.removeEventListener("keydown", alCapturarTecla, true);
  window.removeEventListener("mousedown", alCapturarMouse, true);
  vozPintarOpciones();
}

// Tecla virtual de Windows (VK_*) de un evento de teclado; Shift/Ctrl/Alt con su lado (izquierdo o derecho).
function vkDe(e) {
  const der = e.location === 2;
  if (e.keyCode === 16) return der ? 0xA1 : 0xA0;
  if (e.keyCode === 17) return der ? 0xA3 : 0xA2;
  if (e.keyCode === 18) return der ? 0xA5 : 0xA4;
  return e.keyCode;
}

// En la voz con "tecla para hablar", esa tecla no escribe en el launcher (si es Espacio, no mete espacios en el chat).
// Go la sigue leyendo igual (GetAsyncKeyState), así que habla aunque el launcher tenga el foco.
function tragarTeclaHablar(e) {
  if (capturando || !voz.audio || cfgVoz().vozModo === 1) return;
  if (vkDe(e) !== cfgVoz().vozTecla) return;
  e.preventDefault();
  e.stopPropagation();
}
window.addEventListener("keydown", tragarTeclaHablar, true);
window.addEventListener("keypress", tragarTeclaHablar, true);

function alCapturarTecla(e) {
  e.preventDefault();
  e.stopPropagation();
  if (e.key === "Escape") { terminarCaptura(); return; }
  const vk = vkDe(e);
  let nombre;
  const lado = e.location === 2 ? " der." : " izq.";
  if (e.keyCode === 16) nombre = "Shift" + lado;
  else if (e.keyCode === 17) nombre = "Ctrl" + lado;
  else if (e.keyCode === 18) nombre = e.location === 2 ? "Alt Gr" : "Alt";
  else nombre = NOMBRES_TECLA[e.key] || (e.code.startsWith("Numpad") ? "Num " + e.key : (e.key.length === 1 ? e.key.toUpperCase() : e.key));
  if (!vk || vk > 0xFE) return;
  terminarCaptura();
  guardarVoz({ vozTecla: vk, vozTeclaNombre: nombre });
}

function alCapturarMouse(e) {
  if (e.target === $("op-voz-tecla") && e.button === 0) return;   // el clic que empezó la captura
  e.preventDefault();
  e.stopPropagation();
  const botones = { 1: [0x04, "Mouse 3 (rueda)"], 3: [0x05, "Mouse 4"], 4: [0x06, "Mouse 5"] };
  const b = botones[e.button];
  if (!b) { $("op-voz-tecla").textContent = "El izquierdo y el derecho no"; return; }
  terminarCaptura();
  guardarVoz({ vozTecla: b[0], vozTeclaNombre: b[1] });
}

$("op-voz-tecla").addEventListener("click", () => {
  if (capturando) return;
  capturando = true;
  $("op-voz-tecla").textContent = "Apretá una tecla…";
  $("op-voz-tecla").classList.add("esperando");
  window.addEventListener("keydown", alCapturarTecla, true);
  window.addEventListener("mousedown", alCapturarMouse, true);
});

// Probar micrófono: muestra el nivel durante 15 segundos, con la marca de la sensibilidad.
function pintarMedidor(db) {
  $("voz-medidor-nivel").style.width = dbAPorcentaje(db) + "%";
}

async function probarMicrofono() {
  if (prueba.activa) { terminarPrueba(); return; }
  try {
    if (!voz.audio) {
      prueba.stream = await navigator.mediaDevices.getUserMedia({ audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true } });
      iniciarAnalisis(prueba.stream.getAudioTracks()[0]);
    }
  } catch (err) {
    aviso("error", "No se pudo abrir el micrófono: " + (err && err.message || err));
    return;
  }
  prueba.activa = true;
  prueba.fin = Date.now() + 15000;
  $("op-voz-probar").textContent = "Terminar prueba";
  prueba.intervalo = setInterval(() => {
    pintarMedidor(nivelDb());
    if (Date.now() > prueba.fin || $("opciones").hidden) terminarPrueba();
  }, 50);
}

function terminarPrueba() {
  clearInterval(prueba.intervalo);
  prueba.activa = false;
  if (prueba.stream) { prueba.stream.getTracks().forEach((t) => t.stop()); prueba.stream = null; }
  detenerAnalisis();
  $("op-voz-probar").textContent = "Probar micrófono";
  pintarMedidor(-100);
}

$("op-voz-probar").addEventListener("click", probarMicrofono);
document.querySelector('.seccion[data-seccion="voz"]').addEventListener("click", vozPintarOpciones);

pintarBarraVoz();
