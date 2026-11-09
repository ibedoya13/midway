import { describe, expect, it } from 'vitest'

import { BOARD_SIZE, COLUMN_LABELS, cellLabel, shipCells, shipsOverlap } from './board'

describe('board helpers', () => {
  it('exposes a 10x10 grid with columns A..J', () => {
    expect(BOARD_SIZE).toBe(10)
    expect(COLUMN_LABELS).toHaveLength(10)
    expect(COLUMN_LABELS[0]).toBe('A')
    expect(COLUMN_LABELS[9]).toBe('J')
  })

  describe('cellLabel', () => {
    it('formats a cell as column + row number', () => {
      expect(cellLabel(0, 0)).toBe('A1')
      expect(cellLabel(4, 4)).toBe('E5')
      expect(cellLabel(9, 9)).toBe('J10')
    })
  })

  describe('shipCells', () => {
    it('returns the cells of a horizontal ship', () => {
      expect(shipCells({ x: 2, y: 3, size: 3, orientation: 'horizontal' })).toEqual([
        { x: 2, y: 3 },
        { x: 3, y: 3 },
        { x: 4, y: 3 },
      ])
    })

    it('returns the cells of a vertical ship', () => {
      expect(shipCells({ x: 1, y: 5, size: 2, orientation: 'vertical' })).toEqual([
        { x: 1, y: 5 },
        { x: 1, y: 6 },
      ])
    })
  })

  describe('shipsOverlap', () => {
    it('detects overlapping ships', () => {
      const a = { x: 0, y: 0, size: 3, orientation: 'horizontal' }
      const b = { x: 2, y: 0, size: 3, orientation: 'horizontal' }
      expect(shipsOverlap(a, b)).toBe(true)
    })

    it('detects ships that do not overlap', () => {
      const a = { x: 0, y: 0, size: 3, orientation: 'horizontal' }
      const b = { x: 5, y: 5, size: 2, orientation: 'vertical' }
      expect(shipsOverlap(a, b)).toBe(false)
    })

    it('detects a vertical ship crossing a horizontal one', () => {
      const a = { x: 0, y: 0, size: 3, orientation: 'horizontal' }
      const b = { x: 1, y: -1, size: 3, orientation: 'vertical' }
      expect(shipsOverlap(a, b)).toBe(true)
    })
  })
})
