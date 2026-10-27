// Puntero del mouse: la mano (el guantelete) de Diablo II en toda la ventana, como en el juego. La saca el launcher
// de los archivos del juego del jugador (cursor.go: img/cursor/mano.json y mano-N.png), así que hasta que la tenga
// (por ejemplo, antes de elegir la carpeta del juego) queda el puntero de Windows. Al apretar el botón la mano se
// abre como en el juego (cuadros 1 a 7) y al soltarlo vuelve a señalar. En los campos de texto queda la barrita.
(() => {
  const raiz = document.documentElement;
  const BASE = "img/cursor/";
  const MS_CUADRO = 40; // el juego anima a 25 cuadros por segundo
  let info = null;      // { cuadros, x, y }
  let valores = [];     // el valor de `cursor` de cada cuadro
  let apretado = false;
  let reloj = 0;

  const cargarImagen = (src) => new Promise((ok, mal) => {
    const img = new Image();
    img.onload = () => ok(img);
    img.onerror = mal;
    img.src = src;
  });

  function poner(cuadro) {
    if (valores[cuadro]) raiz.style.setProperty("--mano", valores[cuadro]);
  }

  async function cargar() {
    try {
      const r = await fetch(BASE + "mano.json", { cache: "no-store" });
      if (!r.ok) return false;
      const i = await r.json();
      if (!(i.cuadros > 0)) return false;
      const urls = [];
      for (let n = 0; n < i.cuadros; n++) urls.push([`${BASE}mano-${n}.png`, `${BASE}mano-${n}@2x.png`]);
      // todas cargadas antes de usarlas: así la animación no pasa por un cuadro vacío
      await Promise.all(urls.flat().map(cargarImagen));
      valores = urls.map(([u1, u2]) => `image-set(url("${u1}") 1x, url("${u2}") 2x) ${i.x} ${i.y}, auto`);
      // si este Chromium no entiende image-set en `cursor`, la de 1x sola
      if (!CSS.supports("cursor", valores[0])) valores = urls.map(([u1]) => `url("${u1}") ${i.x} ${i.y}, auto`);
      info = i;
      poner(0);
      raiz.classList.add("mano");
      return true;
    } catch (e) {
      return false;
    }
  }

  function animar() {
    clearInterval(reloj);
    let cuadro = 0;
    reloj = setInterval(() => {
      if (!apretado || cuadro >= info.cuadros - 1) { clearInterval(reloj); return; }
      poner(++cuadro); // se queda en el último mientras siga apretado
    }, MS_CUADRO);
  }

  function soltar() {
    if (!apretado) return;
    apretado = false;
    clearInterval(reloj);
    poner(0);
  }

  window.addEventListener("pointerdown", (e) => {
    if (!info || e.button !== 0) return;
    apretado = true;
    animar();
  }, true);
  window.addEventListener("pointerup", soltar, true);
  window.addEventListener("pointercancel", soltar, true);
  // al arrastrar la ventana el sistema se queda con el soltar: se nota en el próximo movimiento sin botones
  window.addEventListener("pointermove", (e) => { if (apretado && !(e.buttons & 1)) soltar(); }, true);
  window.addEventListener("blur", soltar);

  // Primer intento al abrir; si todavía no hay juego (o falla), se reintenta cada tanto y al volver a la ventana.
  let intentos = 0;
  async function intentar() {
    if (info || intentos++ > 60) return;
    if (!(await cargar())) setTimeout(intentar, 15000);
  }
  window.addEventListener("focus", () => { if (!info) cargar(); });
  intentar();
})();
