"use strict";

// Idioma del launcher (Configuración → Acerca de): español o inglés. Se escribe todo en castellano (HTML, app.js,
// voz.js y los mensajes que manda el lado Go) y, en inglés, esta capa traduce lo que aparece en la ventana:
//   - T(texto): la traducción de un texto (exacta, o por patrón si tiene partes variables);
//   - un MutationObserver traduce cada texto y cada title/placeholder/alt/aria-label que se agrega o cambia, y guarda
//     el original en castellano para poder volver;
//   - lo marcado con translate="no" no se toca: nombres de jugadores y canales, lo que escribe la gente y los
//     mensajes del servidor (Battle.net ya los manda en el idioma del launcher; ver UsarIdioma en app.go).

const IDIOMA_GUARDADO = "idioma-launcher";
let idioma = (() => {
  try { return localStorage.getItem(IDIOMA_GUARDADO) === "en" ? "en" : "es"; } catch (_) { return "es"; }
})();

const EN = {
  // barra, inicio y botón grande
  "Inicio": "Home", "Chat": "Chat", "Configuración": "Settings", "Minimizar": "Minimize", "Cerrar": "Close",
  "Revisando…": "Checking…", "Cambiar la carpeta del juego": "Change the game folder", "Carpeta del juego": "Game folder",
  "Elegir…": "Choose…", "Elegir la carpeta del juego": "Choose the game folder", "Jugar": "Play", "Actualizar": "Update",
  "Instalar LoD": "Install LoD", "Novedades": "News", "Preparando…": "Getting ready…",
  "Preparando la instalación…": "Preparing the installation…", "Diablo II abierto. ¡Suerte!": "Diablo II is open. Good luck!",
  "Launcher actualizado": "Launcher updated",
  // estado y progreso (app.go, actualizar.go)
  "Nueva versión del launcher": "New launcher version", "Elegí la carpeta de Diablo II": "Choose the Diablo II folder",
  "Faltan archivos del juego": "Game files are missing",
  "Esa carpeta tiene otra versión de Diablo II. OmbuServer necesita la 1.14b.": "That folder has another version of Diablo II. OmbuServer needs 1.14b.",
  "No se pudo conectar con OmbuServer. Revisá tu conexión.": "Could not connect to OmbuServer. Check your connection.",
  "Hay una actualización de OmbuServer.": "There is an OmbuServer update.", "El juego está al día.": "The game is up to date.",
  "Buscando actualizaciones…": "Looking for updates…", "Abriendo la versión nueva…": "Opening the new version…",
  "Revisando el juego…": "Checking the game…", "Actualizando OmbuServer…": "Updating OmbuServer…",
  "Bajando actualización…": "Downloading update…", "Configurando Battle.net…": "Setting up Battle.net…",
  "Abriendo Diablo II…": "Opening Diablo II…", "Actualizando el launcher…": "Updating the launcher…",
  "ya hay una tarea en curso": "another task is already running",
  "en la carpeta elegida no está Game.exe": "Game.exe is not in the chosen folder",
  "la carpeta elegida tiene otra versión de Diablo II: OmbuServer necesita la 1.14b": "the chosen folder has another version of Diablo II: OmbuServer needs 1.14b",
  "no se pudo conectar con OmbuServer: revisá tu conexión y volvé a intentar": "could not connect to OmbuServer: check your connection and try again",
  "el launcher bajado no coincide con el publicado: volvé a intentar": "the downloaded launcher does not match the published one: try again",
  "el archivo está en uso: cerrá Diablo II y volvé a intentar": "the file is in use: close Diablo II and try again",
  "Bienvenido a OmbuServer": "Welcome to OmbuServer",
  "Diablo II: Lord of Destruction 1.14b con el Mundo 6: Dún Fiacla, escenarios nuevos y misiones con voces. Tocá JUGAR y el launcher se encarga del resto.":
    "Diablo II: Lord of Destruction 1.14b with World 6: Dún Fiacla, new areas and voiced quests. Press PLAY and the launcher takes care of the rest.",
  // configuración
  "Juego": "Game", "Al jugar": "When playing", "Voz": "Voice", "Acerca de": "About", "Guardado": "Saved",
  "Carpeta de Diablo II": "Diablo II folder",
  "La carpeta donde está Game.exe (Diablo II: Lord of Destruction 1.14b).": "The folder where Game.exe is (Diablo II: Lord of Destruction 1.14b).",
  "Sin elegir": "Not chosen", "Cambiar…": "Change…", "Abrir": "Open", "Diablo II 1.14b encontrado.": "Diablo II 1.14b found.",
  "Ventanas de Diablo II": "Diablo II windows", "Cuántos juegos se abren al tocar JUGAR.": "How many games open when you press PLAY.",
  "Menos": "Less", "Más": "More", "Espera entre ventanas": "Delay between windows",
  "Segundos entre un juego y el siguiente.": "Seconds between one game and the next.", "Modo ventana": "Windowed mode",
  "Abre el juego en una ventana en lugar de pantalla completa.": "Opens the game in a window instead of full screen.",
  "Sonido": "Sound", "Sonido y música del juego.": "Game sound and music.", "Idioma del juego": "Game language",
  "Textos y menús del juego.": "In-game texts and menus.", "Tamaño de ventana": "Window size",
  "El juego se ve igual para todos, solo más grande.": "The game looks the same for everyone, just bigger.",
  "La voz anda en los canales privados: creá uno con": "Voice works in private channels: create one with",
  "/join nombre": "/join name", "y tus amigos entran con el mismo comando.": "and your friends join with the same command.",
  "Cómo hablar": "How to talk",
  "Con una tecla (anda aunque estés jugando) o solo, cuando detecta tu voz.": "With a key (works even while you play) or automatically, when it detects your voice.",
  "Tecla para hablar": "Push to talk", "Detección de voz": "Voice detection",
  "Mantenela apretada mientras hablás. Sirve una tecla o un botón del mouse.": "Hold it down while you talk. A key or a mouse button works.",
  "Sensibilidad": "Sensitivity",
  "Más alta: capta la voz baja (y también ruidos). Probá hasta que la barra pase la marca solo cuando hablás.": "Higher: picks up quiet voices (and noise too). Adjust it until the bar passes the mark only when you talk.",
  "Probar micrófono": "Test microphone", "Terminar prueba": "Stop test", "Apretá una tecla…": "Press a key…",
  "El izquierdo y el derecho no": "Not left or right click", "Idioma del launcher": "Launcher language",
  "Textos del launcher y mensajes del chat.": "Launcher texts and chat messages.",
  "Versión del launcher": "Launcher version", "Servidor": "Server",
  // nombres de teclas (voz.js)
  "Espacio": "Space", "Bloq Mayús": "Caps Lock", "Retroceso": "Backspace", "Supr": "Delete", "Fin": "End",
  "Re Pág": "Page Up", "Av Pág": "Page Down", "Flecha arriba": "Up Arrow", "Flecha abajo": "Down Arrow",
  "Flecha izquierda": "Left Arrow", "Flecha derecha": "Right Arrow", "Pausa": "Pause", "Bloq Despl": "Scroll Lock",
  "Menú": "Menu", "Mouse 3 (rueda)": "Mouse 3 (wheel)", "la tecla": "the key",
  // chat
  "Chat de Battle.net": "Battle.net chat", "Cuenta": "Account", "Contraseña": "Password", "Recordarme": "Remember me",
  "Entrar": "Enter", "Entrando…": "Signing in…", "Cambiar de canal": "Change channel", "Nombre del canal": "Channel name",
  "Cerrar la sesión del chat": "Sign out of the chat", "Salir": "Sign out", "Escribí un mensaje…": "Type a message…",
  "Anterior": "Previous", "Siguiente": "Next", "Página anterior": "Previous page", "Página siguiente": "Next page",
  "En pausa mientras jugás con esta cuenta": "Paused while you play with this account",
  "Chat en pausa mientras jugás con esta cuenta.": "Chat paused while you play with this account.",
  "Completá la cuenta y la contraseña": "Enter your account and password",
  "completá la cuenta y la contraseña": "enter your account and password",
  "Se cortó la conexión con Battle.net. Reconectando…": "Lost the connection to Battle.net. Reconnecting…",
  "sesión cancelada": "session canceled", "no estás conectado al chat": "you are not connected to the chat",
  "la cuenta o la contraseña no son correctas": "wrong account or password",
  "respuesta inválida del servidor": "invalid response from the server",
  "no se pudo conectar con Battle.net de OmbuServer": "could not connect to OmbuServer's Battle.net",
  "Battle.net cortó la conexión": "Battle.net closed the connection",
  "el servidor no aceptó la versión del cliente de chat": "the server did not accept the chat client version",
  "esa cuenta está cerrada": "that account is closed",
  "Susurrar": "Whisper", "Dejar de susurrarle": "Stop whispering", "Ignorar": "Ignore", "No ignorar": "Unignore",
  "Mutear": "Mute", "Desmutear": "Unmute", "Mutear para todos": "Mute for everyone", "Desmutear para todos": "Unmute for everyone",
  "Admin": "Admin", "Operador": "Operator", "Operador del canal": "Channel operator", "Ignorado": "Ignored",
  "Amazona": "Amazon", "Hechicera": "Sorceress", "Nigromante": "Necromancer", "Paladín": "Paladin",
  "Bárbaro": "Barbarian", "Druida": "Druid", "Asesina": "Assassin",
  // voz
  "Entrar a la voz del canal": "Join the channel's voice chat", "Silenciar micrófono": "Mute microphone",
  "Activar micrófono": "Unmute microphone", "Probar el micrófono de nuevo": "Try the microphone again",
  "Salir de la voz": "Leave voice", "Reconectando la voz…": "Reconnecting voice…", "Entrando a la voz…": "Joining voice…",
  "Sin micrófono: solo escuchás": "No microphone: listening only",
  "El operador del canal te sacó la voz": "The channel operator muted you", "Micrófono silenciado": "Microphone muted",
  "Detección de voz: hablá y te escuchan": "Voice detection: just talk and they hear you",
  "Entraste a la voz del canal.": "You joined the channel's voice chat.", "Se cortó la voz.": "Voice disconnected.",
  "El micrófono no está disponible en este equipo.": "The microphone is not available on this computer.",
  "No se encontró un micrófono: entraste a la voz solo para escuchar.": "No microphone found: you joined voice to listen only.",
  "Windows no deja usar el micrófono: Configuración de Windows → Privacidad y seguridad → Micrófono → activá \"Permitir que las aplicaciones de escritorio accedan al micrófono\". Mientras tanto, solo escuchás.":
    "Windows is blocking the microphone: Windows Settings → Privacy & security → Microphone → turn on \"Let desktop apps access your microphone\". Meanwhile, you can only listen.",
  "El micrófono está ocupado por otro programa: entraste solo para escuchar.": "The microphone is being used by another program: you joined to listen only.",
  "no se pudo abrir el micrófono": "could not open the microphone",
  "entrá al chat para usar la voz": "sign in to the chat to use voice",
  "no se pudo conectar con el servidor de voz": "could not connect to the voice server",
  "demasiados intentos: esperá un minuto": "too many attempts: wait a minute",
  "respuesta inválida del servidor de voz": "invalid response from the voice server",
  "el servidor de voz no dio permiso": "the voice server did not allow it",
  "cuenta o contraseña incorrectas": "wrong account or password", "sala inválida": "invalid room",
  "la cuenta está bloqueada": "the account is locked",
  "la voz es solo para canales privados: creá uno con /join <nombre>": "voice is only for private channels: create one with /join <name>",
  "la voz no está disponible ahora (el servidor no informa los canales)": "voice is not available right now (the server is not reporting channels)",
  "tenés que estar en el canal para usar su voz": "you have to be in the channel to use its voice chat",
  "solo el operador del canal puede silenciar para todos": "only the channel operator can mute for everyone",
  "a quién silenciar no es válido": "invalid user to mute", "no se puede silenciar a un administrador": "an administrator cannot be muted",
  "pedido inválido": "invalid request", "acción inválida": "invalid action",
};

// Textos con partes variables: [patrón, traducción]. En la traducción, $1.. son las partes tal cual y T$1.. las
// partes traducidas (por ejemplo, el nombre de una tecla).
const EN_PATRONES = [
  [/^Abriendo Diablo II \((\d+) de (\d+)\)…$/, "Opening Diablo II ($1 of $2)…"],
  [/^No se puede leer esa carpeta: (.*)$/s, "Can't read that folder: $1"],
  [/^no se pudo configurar Battle\.net: (.*)$/s, "could not set up Battle.net: $1"],
  [/^no se pudo abrir Game\.exe: (.*)$/s, "could not open Game.exe: $1"],
  [/^no se pudo reemplazar el launcher \(¿está en una carpeta protegida\?\): (.*)$/s, "could not replace the launcher (is it in a protected folder?): $1"],
  [/^no se pudo reemplazar el launcher: (.*)$/s, "could not replace the launcher: $1"],
  [/^el launcher se actualizó, pero no se pudo abrir: abrilo de nuevo \((.*)\)$/s, "the launcher was updated, but could not be opened: open it again ($1)"],
  [/^version\.json del launcher: (.*)$/s, "launcher version.json: $1"],
  [/^el MPQ bajado no coincide con la versión publicada (.*)$/s, "the downloaded MPQ does not match the published version $1"],
  [/^versión publicada inválida: (.*)$/s, "invalid published version: $1"],
  [/^manifiesto del cliente: (.*)$/s, "client manifest: $1"],
  [/^el manifiesto trae archivos no permitidos en la carpeta del juego: (.*)$/s, "the manifest has files that are not allowed in the game folder: $1"],
  [/^(.*): CRC bajado (\S+) distinto del publicado (\S+)$/s, "$1: downloaded CRC $2 differs from the published $3"],
  [/^No se pudo volver a entrar al chat: (.*)$/s, "Could not rejoin the chat: T$1"],
  [/^Battle\.net rechazó el inicio de sesión \(código (\d+)\)$/, "Battle.net rejected the sign-in (code $1)"],
  [/^el servidor de voz respondió (\d+)$/, "the voice server replied $1"],
  [/^LiveKit respondió (\d+)$/, "LiveKit replied $1"],
  [/^no se pudo hablar con el servidor de voz: (.*)$/s, "could not reach the voice server: $1"],
  [/^No se pudo entrar a la voz: (.*)$/s, "Could not join voice: T$1"],
  [/^No se pudo abrir el micrófono: (.*)$/s, "Could not open the microphone: $1"],
  [/^No se pudo abrir el micrófono \((.*)\): entraste solo para escuchar\.$/s, "Could not open the microphone ($1): you joined to listen only."],
  [/^Mantené (.+) para hablar$/, "Hold T$1 to talk"],
  [/^(\d+) en la voz$/, "$1 in voice"],
  [/^nivel (\d+)$/, "level $1"],
  [/^cuenta (.+)$/, "account $1"],
  [/^Susurrando a (.+)…$/, "Whispering to $1…"],
  [/^De <(.*)>: $/, "From <$1>: "],
  [/^Para <(.*)>: $/, "To <$1>: "],
  [/^Entraste al canal: (.*)$/, "You joined the channel: $1"],
  [/^El canal (.*) está lleno\.$/, "The channel $1 is full."],
  [/^El canal (.*) no existe\.$/, "The channel $1 does not exist."],
  [/^El canal (.*) es restringido\.$/, "The channel $1 is restricted."],
  [/^(.+) no puede hablar en la voz del canal\.$/, "$1 can't talk in the channel's voice chat."],
  [/^(.+) puede volver a hablar en la voz\.$/, "$1 can talk in voice again."],
];

function traducirEn(s) {
  if (Object.prototype.hasOwnProperty.call(EN, s)) return EN[s];
  for (const [re, plantilla] of EN_PATRONES) {
    const m = re.exec(s);
    if (m) return plantilla.replace(/(T?)\$(\d)/g, (_, t, i) => (t ? T(m[i]) : m[i]));
  }
  // los errores llegan con la primera letra en mayúscula (app.js): se busca en minúscula y se vuelve a subir
  const primera = s.charAt(0);
  if (primera !== primera.toLowerCase()) {
    const t = traducirEn(primera.toLowerCase() + s.slice(1));
    if (t !== null) return t.charAt(0).toUpperCase() + t.slice(1);
  }
  return null;
}

// T: el texto en el idioma del launcher. Respeta los espacios de los bordes, las partes separadas por " · " y el
// porcentaje que agrega el botón grande ("Actualizando OmbuServer 45%").
function T(s) {
  if (idioma === "es" || typeof s !== "string") return s;
  const m = /^(\s*)([\s\S]*?)(\s*)$/.exec(s);
  const cuerpo = m[2];
  if (!cuerpo) return s;
  let t = traducirEn(cuerpo);
  if (t === null && cuerpo.includes(" · ")) t = cuerpo.split(" · ").map(T).join(" · ");
  if (t === null) {
    const p = /^(.+) (\d+)%$/.exec(cuerpo);
    if (p) { const b = traducirEn(p[1] + "…"); if (b !== null) t = `${b.replace(/…$/, "")} ${p[2]}%`; }
  }
  return t === null ? s : m[1] + t + m[3];
}

// ---------- traducción de la ventana ----------
const ATRIBUTOS_TRADUCIBLES = ["title", "placeholder", "alt", "aria-label"];
const textoOriginal = new WeakMap();   // nodo de texto -> { es: original, puesto: lo último que escribimos }
const atributoOriginal = new WeakMap(); // elemento -> { [atributo]: { es, puesto } }

function noTraducir(nodo) {
  const e = nodo.nodeType === 1 ? nodo : nodo.parentElement;
  return !e || !!e.closest('[translate="no"], script, style');
}

function traducirNodoTexto(n) {
  if (noTraducir(n)) return;
  const actual = n.data;
  let r = textoOriginal.get(n);
  if (!r || r.puesto !== actual) r = { es: actual };   // lo cambió el código: ese es el original nuevo
  const t = T(r.es);
  r.puesto = t;
  textoOriginal.set(n, r);
  if (t !== actual) n.data = t;
}

function traducirAtributo(e, nombre) {
  if (!e.hasAttribute(nombre) || noTraducir(e)) return;
  const actual = e.getAttribute(nombre);
  const todos = atributoOriginal.get(e) || {};
  let r = todos[nombre];
  if (!r || r.puesto !== actual) r = { es: actual };
  const t = T(r.es);
  r.puesto = t;
  todos[nombre] = r;
  atributoOriginal.set(e, todos);
  if (t !== actual) e.setAttribute(nombre, t);
}

function traducirArbol(raiz) {
  if (raiz.nodeType === 3) { traducirNodoTexto(raiz); return; }
  if (raiz.nodeType !== 1) return;
  const recorrido = document.createTreeWalker(raiz, NodeFilter.SHOW_TEXT | NodeFilter.SHOW_ELEMENT);
  for (let n = raiz; n; n = recorrido.nextNode()) {
    if (n.nodeType === 3) { if (n.data.trim()) traducirNodoTexto(n); }
    else for (const a of ATRIBUTOS_TRADUCIBLES) traducirAtributo(n, a);
  }
}

new MutationObserver((cambios) => {
  for (const c of cambios) {
    if (c.type === "childList") c.addedNodes.forEach(traducirArbol);
    else if (c.type === "characterData") traducirNodoTexto(c.target);
    else if (c.type === "attributes") traducirAtributo(c.target, c.attributeName);
  }
}).observe(document.body, { subtree: true, childList: true, characterData: true, attributes: true, attributeFilter: ATRIBUTOS_TRADUCIBLES });

function aplicarIdioma() {
  document.documentElement.lang = idioma;
  traducirArbol(document.body);
  document.querySelectorAll("#op-idioma-launcher button").forEach((b) => b.classList.toggle("activo", b.dataset.idiomaLauncher === idioma));
}

// cambiarIdioma: lo llama la sección Acerca de. Lo ya escrito en el chat queda como estaba.
function cambiarIdioma(cod) {
  idioma = cod === "en" ? "en" : "es";
  try { localStorage.setItem(IDIOMA_GUARDADO, idioma); } catch (_) { /* sin almacenamiento: vale hasta cerrar */ }
  aplicarIdioma();
  document.dispatchEvent(new Event("idioma"));
}

aplicarIdioma();
