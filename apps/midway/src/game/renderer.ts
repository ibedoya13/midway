/**
 * Pixel art renderer for the Battleship board.
 *
 * The board is drawn on a 2D canvas with hard-edged rectangles only, so it
 * scales crisply when the canvas is stretched with `image-rendering: pixelated`.
 */

import { COLUMN_LABELS } from './board'
import { normalizeBoard } from './normalize'
import type { BoardView, ShipView } from './types'

/** Size in logical pixels of a single board cell. */
export const TILE_SIZE = 24

/** Palette used by the board renderer. */
export const PALETTE = {
  sea: '#12304a',
  seaDark: '#0e2740',
  grid: '#1d4a70',
  outline: '#0c0e14',
  red: '#e94f6d',
  blue: '#4f8ef7',
  water: '#9ad9ff',
  hit: '#ffd447',
  sunk: '#ff6b8a',
  label: '#7fa8c9',
  labelBack: '#0c0e14',
  highlight: '#ffffff',
  hover: '#2a5a82',
} as const

/** 3x5 pixel font, enough for columns A-J and rows 1-10. */
const FONT: Record<string, string[]> = {
  A: ['010', '101', '111', '101', '101'],
  B: ['110', '101', '110', '101', '110'],
  C: ['011', '100', '100', '100', '011'],
  D: ['110', '101', '101', '101', '110'],
  E: ['111', '100', '110', '100', '111'],
  F: ['111', '100', '110', '100', '100'],
  G: ['011', '100', '101', '101', '011'],
  H: ['101', '101', '111', '101', '101'],
  I: ['111', '010', '010', '010', '111'],
  J: ['001', '001', '001', '101', '010'],
  '0': ['111', '101', '101', '101', '111'],
  '1': ['010', '110', '010', '010', '111'],
  '2': ['110', '001', '010', '100', '111'],
  '3': ['110', '001', '010', '001', '110'],
  '4': ['101', '101', '111', '001', '001'],
  '5': ['111', '100', '110', '001', '110'],
  '6': ['011', '100', '111', '101', '111'],
  '7': ['111', '001', '010', '010', '010'],
  '8': ['111', '101', '111', '101', '111'],
  '9': ['111', '101', '111', '001', '110'],
}

export interface RenderOptions {
  /** Draw the ships of the board (true for the own board). */
  showShips: boolean
  /** Cell currently selected. */
  highlight?: { x: number; y: number } | null
  /** Cell under the pointer. */
  hover?: { x: number; y: number } | null
  /** Color of the side owning the board: used for the ships and to tint the sea. */
  color?: string
}

/**
 * tint blends a base color with an accent color.
 *
 * @param base hex color to blend into
 * @param accent hex color to blend with
 * @param amount weight of the accent, from 0 to 1
 */
export function tint(base: string, accent: string, amount: number): string {
  const parse = (hex: string): [number, number, number] => {
    const clean = hex.replace('#', '')
    const full =
      clean.length === 3
        ? clean
            .split('')
            .map((c) => c + c)
            .join('')
        : clean
    return [
      parseInt(full.slice(0, 2), 16),
      parseInt(full.slice(2, 4), 16),
      parseInt(full.slice(4, 6), 16),
    ]
  }
  const [r1, g1, b1] = parse(base)
  const [r2, g2, b2] = parse(accent)
  const mix = (a: number, b: number) =>
    Math.round(a + (b - a) * amount)
      .toString(16)
      .padStart(2, '0')
  return `#${mix(r1, r2)}${mix(g1, g2)}${mix(b1, b2)}`
}

/**
 * canvasSize returns the logical pixel size needed to draw a board.
 */
export function canvasSize(width: number, height: number): { width: number; height: number } {
  return { width: width * TILE_SIZE, height: height * TILE_SIZE }
}

/**
 * pixelToTile converts logical canvas pixels into board coordinates.
 * It returns null when the point falls outside the board.
 */
export function pixelToTile(
  px: number,
  py: number,
  width = 10,
  height = 10,
): { x: number; y: number } | null {
  const x = Math.floor(px / TILE_SIZE)
  const y = Math.floor(py / TILE_SIZE)
  if (x < 0 || y < 0 || x >= width || y >= height) return null
  return { x, y }
}

/**
 * renderBoard draws a full board on the given 2D context.
 */
export function renderBoard(
  ctx: CanvasRenderingContext2D,
  board: BoardView,
  opts: RenderOptions,
): void {
  // Guard against a malformed board: a null list would throw on forEach and
  // break the whole render.
  const safe = normalizeBoard(board)
  const width = safe.width || 10
  const height = safe.height || 10
  const color = opts.color ?? PALETTE.red

  ctx.imageSmoothingEnabled = false

  drawSea(ctx, width, height, color)
  drawGrid(ctx, width, height)

  if (opts.showShips) {
    safe.ships.forEach((ship) => drawShip(ctx, ship, color, false))
  }

  safe.sunkShips.forEach((ship) => drawShip(ctx, ship, color, true))

  safe.shots.forEach((shot) => {
    if (shot.result === 'water') drawWater(ctx, shot.x, shot.y)
    else if (shot.result === 'hit') drawHit(ctx, shot.x, shot.y)
    else drawSunk(ctx, shot.x, shot.y)
  })

  drawLabels(ctx, width, height)

  if (opts.hover) {
    drawCellOverlay(ctx, opts.hover.x, opts.hover.y, PALETTE.hover)
  }
  if (opts.highlight) {
    drawHighlight(ctx, opts.highlight.x, opts.highlight.y)
  }
}

function drawSea(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  color: string,
): void {
  // Each side sees its own sea tinted with its color, so the two boards are
  // told apart even when no ship is visible.
  ctx.fillStyle = tint(PALETTE.sea, color, 0.22)
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      ctx.fillRect(x * TILE_SIZE, y * TILE_SIZE, TILE_SIZE, TILE_SIZE)
    }
  }

  ctx.fillStyle = tint(PALETTE.seaDark, color, 0.22)
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      if ((x + y) % 2 !== 0) continue
      ctx.fillRect(x * TILE_SIZE + 4, y * TILE_SIZE + 6, 2, 2)
      ctx.fillRect(x * TILE_SIZE + 12, y * TILE_SIZE + 14, 2, 2)
      ctx.fillRect(x * TILE_SIZE + 18, y * TILE_SIZE + 4, 2, 2)
    }
  }
}

function drawGrid(ctx: CanvasRenderingContext2D, width: number, height: number): void {
  ctx.fillStyle = PALETTE.grid
  for (let x = 0; x <= width; x++) {
    ctx.fillRect(x * TILE_SIZE, 0, 1, height * TILE_SIZE)
  }
  for (let y = 0; y <= height; y++) {
    ctx.fillRect(0, y * TILE_SIZE, width * TILE_SIZE, 1)
  }
}

function drawShip(
  ctx: CanvasRenderingContext2D,
  ship: ShipView,
  color: string,
  sunk: boolean,
): void {
  const vertical = ship.orientation === 'vertical'
  for (let i = 0; i < ship.size; i++) {
    const x = vertical ? ship.x : ship.x + i
    const y = vertical ? ship.y + i : ship.y
    const px = x * TILE_SIZE
    const py = y * TILE_SIZE

    if (sunk) {
      ctx.fillStyle = PALETTE.outline
      ctx.fillRect(px, py, TILE_SIZE, TILE_SIZE)
      ctx.fillStyle = color
      ctx.fillRect(px + 3, py + 3, TILE_SIZE - 6, TILE_SIZE - 6)
      ctx.fillStyle = PALETTE.outline
      ctx.fillRect(px, py + Math.floor(TILE_SIZE / 2) - 1, TILE_SIZE, 2)
    } else {
      ctx.fillStyle = PALETTE.outline
      ctx.fillRect(px + 2, py + 2, TILE_SIZE - 4, TILE_SIZE - 4)
      ctx.fillStyle = color
      ctx.fillRect(px + 4, py + 4, TILE_SIZE - 8, TILE_SIZE - 8)
      ctx.fillStyle = PALETTE.outline
      ctx.fillRect(px + 7, py + 7, 2, 2)
      ctx.fillRect(px + 13, py + 11, 2, 2)
    }
  }
}

function drawWater(ctx: CanvasRenderingContext2D, x: number, y: number): void {
  const px = x * TILE_SIZE
  const py = y * TILE_SIZE
  ctx.fillStyle = PALETTE.water
  // Hollow circle (the "o" of the rules).
  ctx.fillRect(px + 8, py + 7, 8, 2)
  ctx.fillRect(px + 8, py + 15, 8, 2)
  ctx.fillRect(px + 6, py + 9, 2, 6)
  ctx.fillRect(px + 16, py + 9, 2, 6)
}

function drawHit(ctx: CanvasRenderingContext2D, x: number, y: number): void {
  const px = x * TILE_SIZE
  const py = y * TILE_SIZE
  ctx.fillStyle = PALETTE.hit
  // Diagonal cross (the "x" of the rules).
  for (let i = 0; i < 10; i++) {
    ctx.fillRect(px + 7 + i, py + 7 + i, 2, 2)
    ctx.fillRect(px + 16 - i, py + 7 + i, 2, 2)
  }
}

function drawSunk(ctx: CanvasRenderingContext2D, x: number, y: number): void {
  const px = x * TILE_SIZE
  const py = y * TILE_SIZE
  ctx.fillStyle = PALETTE.sunk
  // Solid square (the filled square of the rules).
  ctx.fillRect(px + 6, py + 6, TILE_SIZE - 12, TILE_SIZE - 12)
}

function drawCellOverlay(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  color: string,
): void {
  ctx.fillStyle = color
  ctx.fillRect(x * TILE_SIZE, y * TILE_SIZE, TILE_SIZE, TILE_SIZE)
}

function drawHighlight(ctx: CanvasRenderingContext2D, x: number, y: number): void {
  const px = x * TILE_SIZE
  const py = y * TILE_SIZE
  ctx.fillStyle = PALETTE.highlight
  ctx.fillRect(px, py, TILE_SIZE, 2)
  ctx.fillRect(px, py + TILE_SIZE - 2, TILE_SIZE, 2)
  ctx.fillRect(px, py, 2, TILE_SIZE)
  ctx.fillRect(px + TILE_SIZE - 2, py, 2, TILE_SIZE)
}

function drawLabels(ctx: CanvasRenderingContext2D, width: number, height: number): void {
  for (let x = 0; x < Math.min(width, COLUMN_LABELS.length); x++) {
    drawGlyph(ctx, COLUMN_LABELS[x], x * TILE_SIZE + 5, 4)
  }
  for (let y = 0; y < Math.min(height, 10); y++) {
    drawGlyph(ctx, String(y + 1), 5, y * TILE_SIZE + 5)
  }
}

/**
 * drawGlyph draws one or more characters of the pixel font. Each character is
 * 3 pixels wide with a 1 pixel gap, so "10" renders as two glyphs.
 */
function drawGlyph(ctx: CanvasRenderingContext2D, text: string, px: number, py: number): void {
  const chars = [...text]
  const width = chars.length * 4 - 1

  ctx.fillStyle = PALETTE.labelBack
  ctx.fillRect(px - 1, py - 1, width * 2 + 3, 7)

  ctx.fillStyle = PALETTE.label
  chars.forEach((char, index) => {
    const rows = FONT[char]
    if (!rows) return
    const offset = index * 4
    rows.forEach((row, ry) => {
      for (let rx = 0; rx < row.length; rx++) {
        if (row[rx] === '1') ctx.fillRect(px + offset * 2 + rx * 2, py + ry * 2, 2, 2)
      }
    })
  })
}
