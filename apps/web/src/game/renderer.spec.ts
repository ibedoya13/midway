import { describe, expect, it, vi } from 'vitest'

import { BOARD_SIZE } from './board'
import { TILE_SIZE, canvasSize, pixelToTile, renderBoard } from './renderer'
import type { BoardView } from './types'

interface RecordedRect {
  color: string
  x: number
  y: number
  w: number
  h: number
}

function makeCtx(): { ctx: CanvasRenderingContext2D; rects: RecordedRect[] } {
  const rects: RecordedRect[] = []
  const ctx = {
    imageSmoothingEnabled: true,
    fillStyle: '#000000',
    fillRect: vi.fn((x: number, y: number, w: number, h: number) => {
      rects.push({ color: String(ctx.fillStyle), x, y, w, h })
    }),
  }
  return { ctx: ctx as unknown as CanvasRenderingContext2D, rects }
}

function emptyBoard(): BoardView {
  return {
    width: BOARD_SIZE,
    height: BOARD_SIZE,
    ships: [],
    sunkShips: [],
    shots: [],
  }
}

function boardWithShips(): BoardView {
  return {
    ...emptyBoard(),
    ships: [
      { kind: 'carrier', name: 'Portaaviones', size: 3, x: 1, y: 2, orientation: 'horizontal' },
    ],
  }
}

describe('canvasSize', () => {
  it('returns the logical pixel size of the board', () => {
    expect(canvasSize(10, 10)).toEqual({ width: 10 * TILE_SIZE, height: 10 * TILE_SIZE })
  })
})

describe('pixelToTile', () => {
  it('maps pixels to the tile they fall into', () => {
    expect(pixelToTile(0, 0)).toEqual({ x: 0, y: 0 })
    expect(pixelToTile(TILE_SIZE + 3, TILE_SIZE * 2 + 5)).toEqual({ x: 1, y: 2 })
  })

  it('returns null outside the board', () => {
    expect(pixelToTile(-1, 0)).toBeNull()
    expect(pixelToTile(0, BOARD_SIZE * TILE_SIZE)).toBeNull()
  })
})

describe('renderBoard', () => {
  it('draws the sea background over the whole canvas', () => {
    const { ctx, rects } = makeCtx()
    renderBoard(ctx, emptyBoard(), { showShips: false })
    expect(ctx.imageSmoothingEnabled).toBe(false)
    expect(rects.length).toBeGreaterThan(0)
    const total = rects.reduce((sum, r) => sum + r.w * r.h, 0)
    expect(total).toBeGreaterThanOrEqual(10 * TILE_SIZE * 10 * TILE_SIZE)
  })

  it('draws every cell of the grid', () => {
    const { ctx, rects } = makeCtx()
    renderBoard(ctx, emptyBoard(), { showShips: false })
    const cells = rects.filter((r) => r.w === TILE_SIZE && r.h === TILE_SIZE)
    expect(cells.length).toBeGreaterThanOrEqual(BOARD_SIZE * BOARD_SIZE)
  })

  it('does not draw enemy ships when showShips is false', () => {
    const { ctx, rects } = makeCtx()
    renderBoard(ctx, boardWithShips(), { showShips: false })
    expect(rects.some((r) => r.color === '#e94f6d')).toBe(false)
    expect(rects.some((r) => r.color === '#4f8ef7')).toBe(false)
  })

  it('draws own ships when showShips is true', () => {
    const { ctx, rects } = makeCtx()
    renderBoard(ctx, boardWithShips(), { showShips: true })
    expect(rects.some((r) => r.color === '#e94f6d')).toBe(true)
  })

  it('draws shots: water, hit and sunk', () => {
    const { ctx, rects } = makeCtx()
    const board: BoardView = {
      ...emptyBoard(),
      shots: [
        { x: 0, y: 0, result: 'water' },
        { x: 1, y: 0, result: 'hit' },
        { x: 2, y: 0, result: 'sunk' },
      ],
    }
    renderBoard(ctx, board, { showShips: false })
    expect(rects.some((r) => r.color === '#9ad9ff')).toBe(true)
    expect(rects.some((r) => r.color === '#ffd447')).toBe(true)
    expect(rects.some((r) => r.color === '#ff6b8a')).toBe(true)
  })

  it('labels every column and every row, including row 10', () => {
    const { ctx, rects } = makeCtx()
    renderBoard(ctx, emptyBoard(), { showShips: false })
    const labelRects = rects.filter((r) => r.color === '#7fa8c9')
    // 10 column letters + 10 row numbers, each glyph drawn as pixels.
    expect(labelRects.length).toBeGreaterThan(100)
    // The "10" label sits on the last row, so it must be drawn below y = 9.
    expect(labelRects.some((r) => r.y >= 9 * TILE_SIZE)).toBe(true)
  })

  it('draws the highlight border around the selected cell', () => {
    const { ctx, rects } = makeCtx()
    renderBoard(ctx, emptyBoard(), { showShips: false, highlight: { x: 3, y: 4 } })
    const highlight = rects.filter((r) => r.color === '#ffffff')
    expect(highlight.length).toBeGreaterThan(0)
    expect(highlight.some((r) => r.x === 3 * TILE_SIZE && r.y === 4 * TILE_SIZE)).toBe(true)
  })

  it('draws sunk ships even when ships are hidden', () => {
    const { ctx, rects } = makeCtx()
    const board: BoardView = {
      ...emptyBoard(),
      sunkShips: [
        { kind: 'destroyer', name: 'Destructor', size: 2, x: 5, y: 5, orientation: 'horizontal' },
      ],
    }
    renderBoard(ctx, board, { showShips: false })
    expect(rects.some((r) => r.color === '#0c0e14' && r.x === 5 * TILE_SIZE)).toBe(true)
  })
})
