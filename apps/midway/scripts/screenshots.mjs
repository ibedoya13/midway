/**
 * Automatización de capturas: juega una partida completa y fotografía cada
 * etapa.
 *
 * Uso:
 *   node scripts/screenshots.mjs [urlBase] [directorioDeSalida]
 *
 * Por defecto juega contra http://localhost:5173 y guarda en
 * ../screenshots. Necesita el backend Go y el frontend de Vite levantados:
 *
 *   make dev-server   # o: cd apps/server && go run ./cmd/midway
 *   make dev-web      # o: cd apps/midway && npm run dev
 *
 * El script necesita el paquete `playwright` (devDependency):
 *   npm --workspace apps/midway install --save-dev playwright
 *   npx playwright install chromium
 *
 * La partida dura unos ocho minutos con dos paginas recibiendo el estado
 * 20 veces por segundo. Si el equipo tiene muchos navegadores abiertos, el
 * navegador puede morir al final y quedarse sin las ultimas capturas: cierra
 * los navegadores que tengas abiertos antes de lanzarlo.
 */
import { chromium } from 'playwright'
import { mkdirSync, readdirSync, rmSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const baseUrl = (process.argv[2] ?? 'http://localhost:5173').replace(/\/$/, '')
const outDir = resolve(here, '..', process.argv[3] ?? 'screenshots')

/** Número de casillas de cada lado del tablero. */
const BOARD_SIZE = 10
/** Tamaño lógico de una celda. */
const TILE = 24
/** Margen en píxeles lógicos donde se dibujan las coordenadas. */
const GUTTER = 20

/** La flota que colocan los dos jugadores, en las mismas casillas. */
const FLEET = [
  ['carrier', 0, 0],
  ['battleship', 0, 2],
  ['cruiser', 0, 4],
  ['submarine', 0, 6],
  ['destroyer', 0, 8],
]

/** Las casillas donde empieza cada barco, para disparar contra ellas. */
const TARGETS = [
  [0, 0], [1, 0], [2, 0], [3, 0], [4, 0],
  [0, 2], [1, 2], [2, 2], [3, 2],
  [0, 4], [1, 4], [2, 4],
  [0, 6], [1, 6], [2, 6],
  [0, 8], [1, 8],
]

rmSync(outDir, { recursive: true, force: true })
mkdirSync(outDir, { recursive: true })

/** Hace clic en una celda del tablero. */
async function clickCell(page, boardIndex, x, y) {
  const canvas = page.locator('canvas').nth(boardIndex)
  // At the end of the match the boards are replaced by the result screen.
  if ((await canvas.count()) === 0) return false
  const box = await canvas.boundingBox()
  if (!box) return false
  // The board is scaled to the space it has, so the cell size comes from
  // the rendered canvas and not from the logical size of a tile. The gutter
  // with the coordinates does not belong to the play area.
  const logical = BOARD_SIZE * TILE + GUTTER
  const gutter = (box.width * GUTTER) / logical
  const cell = (box.width - gutter) / BOARD_SIZE
  await page.mouse.click(
    box.x + gutter + x * cell + cell / 2,
    box.y + gutter + y * cell + cell / 2,
  )
  return true
}

/** Coloca la flota completa. */
async function placeFleet(page) {
  for (const [kind, x, y] of FLEET) {
    await page.locator(`[data-ship="${kind}"]`).click()
    await page.waitForTimeout(120)
    await clickCell(page, 0, x, y)
    await page.waitForTimeout(160)
  }
}

/** Espera el turno y dispara. Devuelve false si ya no se puede. */
async function shootAt(page, x, y) {
  if (page.isClosed()) return false
  await page
    .waitForFunction(() => document.body.innerText.includes('Tu turno'), { timeout: 20000 })
    .catch(() => {})
  if (page.isClosed()) return false
  return clickCell(page, 1, x, y)
}

/** Captura una pantalla y la guarda con nombre. */
async function shot(page, name) {
  if (page.isClosed()) {
    console.log(`  ${name}.png OMITIDA: la pagina se cerro`)
    return null
  }
  const file = resolve(outDir, `${name}.png`)
  await page.screenshot({ path: file })
  console.log(`  ${name}.png`)
  return file
}

// The browser is launched with a small footprint: the machine may already
// be running another browser, and the memory pressure kills it.
/** Ejecuta una etapa y anota el fallo sin tumbar la corrida. */
async function stage(name, fn) {
  try {
    await fn()
    return true
  } catch (error) {
    console.log(`  [fallo] ${name}: ${error.message}`)
    return false
  }
}

const browser = await chromium.launch({
  // The full Chromium is used instead of the headless shell: the shell dies
  // when the result screen renders the boards again.
  channel: 'chromium',
  args: ['--disable-gpu', '--disable-dev-shm-usage', '--no-sandbox'],
})
const consoleErrors = []

try {
  // --- Jugador 1: crea la sala ---
  const ctx1 = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1,
  })
  const p1 = await ctx1.newPage()
  p1.on('console', (msg) => {
    if (msg.type() === 'error') consoleErrors.push(`[p1] ${msg.text()}`)
  })

  await p1.goto(baseUrl, { waitUntil: 'networkidle' })
  await p1.waitForTimeout(700)
  await shot(p1, '01-vestibulo')

  await p1.locator('[data-name]').fill('Ana')
  await p1.locator('[data-create]').click()
  await p1.waitForTimeout(1200)
  await shot(p1, '02-sala-creada')

  const invite = await p1.locator('[data-invite-link]').textContent()
  const code = await p1.locator('[data-room-code-display]').textContent()
  console.log(`  sala ${code.trim()}`)

  // --- Jugador 2: se une con el enlace ---
  const ctx2 = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1,
  })
  const p2 = await ctx2.newPage()
  p2.on('console', (msg) => {
    if (msg.type() === 'error') consoleErrors.push(`[p2] ${msg.text()}`)
  })

  await p2.goto(invite.trim(), { waitUntil: 'networkidle' })
  await p2.waitForTimeout(700)
  await shot(p2, '03-vestibulo-con-invitacion')

  await p2.locator('[data-name]').fill('Beto')
  await p2.locator('[data-join-room]').click()
  await p2.waitForTimeout(1200)
  await shot(p2, '04-entro-en-la-sala')

  // --- Colocación de la flota, antes de confirmar ---
  await placeFleet(p1)
  await shot(p1, '05-flota-colocada')
  await placeFleet(p2)
  await shot(p2, '06-flota-colocada-rival')

  // --- Comienza la partida ---
  await p1.locator('[data-ready]').click()
  await p2.locator('[data-ready]').click()
  await p1.waitForTimeout(1500)
  await shot(p1, '07-partida-empezada')

  // --- unos disparos ---
  for (const [x, y] of TARGETS.slice(0, 6)) {
    if (!(await shootAt(p1, x, y))) break
    await p1.waitForTimeout(450)
    if (!(await shootAt(p2, x, y))) break
    await p2.waitForTimeout(450)
  }
  await shot(p1, '08-disparos-en-el-mar-enemigo')
  await shot(p2, '09-disparos-vistos-por-el-rival')

  // --- se hunde un barco entero ---
  for (const [x, y] of TARGETS.slice(0, 5)) {
    if (!(await shootAt(p1, x, y))) break
    await p1.waitForTimeout(450)
    if (!(await shootAt(p2, x, y))) break
    await p2.waitForTimeout(450)
  }
  await shot(p1, '10-barco-hundido')

  // --- se acaba la partida ---
  for (const [x, y] of TARGETS) {
    // Cuando termina, los tableros se sustituyen por la pantalla de
    // resultado: no hay nada mas que disparar.
    const over = await Promise.all([
      p1.locator('[data-gameover]').count(),
      p2.locator('[data-gameover]').count(),
    ])
    if (over[0] > 0 || over[1] > 0) break

    if (!(await shootAt(p1, x, y))) break
    await p1.waitForTimeout(450)
    if (!(await shootAt(p2, x, y))) break
    await p2.waitForTimeout(450)
  }
  await stage('fin de la partida', async () => {
    await p1.waitForTimeout(1200)
    await shot(p1, '11-fin-de-la-partida')
    await shot(p2, '12-fin-de-la-partida-rival')
  })

  // --- el vestíbulo otra vez ---
  // La pantalla de final solo ofrece "Reiniciar", asi que se vuelve al
  // vestibulo recargando la pagina.
  await stage('volver al vestíbulo', async () => {
    await p1.reload({ waitUntil: 'networkidle' })
    await p1.waitForTimeout(1200)
    await shot(p1, '13-de-vuelta-al-vestibulo')
  })

  if (consoleErrors.length > 0) {
    console.log('\nerrores de consola:')
    for (const err of consoleErrors) console.log(`  ${err}`)
  } else {
    console.log('\nsin errores de consola')
  }
} finally {
  await browser.close()
}

const captured = readdirSync(outDir).filter((name) => name.endsWith('.png'))
console.log(`\n${captured.length} capturas en ${outDir}`)
for (const name of captured) console.log(`  ${name}`)
