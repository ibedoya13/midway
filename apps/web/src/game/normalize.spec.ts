import { describe, expect, it } from 'vitest'

import { normalizeSnapshot, normalizeBoard } from './normalize'
import type { BoardView, Snapshot } from './types'

describe('normalizeBoard', () => {
  it('replaces null lists with empty arrays', () => {
    const broken = {
      width: 10,
      height: 10,
      ships: null,
      sunkShips: null,
      shots: null,
    } as unknown as BoardView

    const fixed = normalizeBoard(broken)

    expect(fixed.ships).toEqual([])
    expect(fixed.sunkShips).toEqual([])
    expect(fixed.shots).toEqual([])
  })

  it('keeps the existing lists untouched', () => {
    const board: BoardView = {
      width: 10,
      height: 10,
      ships: [{ kind: 'cruiser', name: 'Crucero', size: 3, x: 1, y: 2, orientation: 'horizontal' }],
      sunkShips: [],
      shots: [{ x: 0, y: 0, result: 'hit' }],
    }

    const fixed = normalizeBoard(board)

    expect(fixed.ships).toHaveLength(1)
    expect(fixed.shots).toHaveLength(1)
  })

  it('falls back to a 10x10 board when the size is missing', () => {
    const broken = { ships: null } as unknown as BoardView

    const fixed = normalizeBoard(broken)

    expect(fixed.width).toBe(10)
    expect(fixed.height).toBe(10)
  })
})

describe('normalizeSnapshot', () => {
  it('repairs both boards of a snapshot with null lists', () => {
    const broken = {
      phase: 'setup',
      width: 10,
      height: 10,
      players: null,
      own: { width: 10, height: 10, ships: null, sunkShips: null, shots: null },
      enemy: { width: 10, height: 10, ships: null, sunkShips: null, shots: null },
      currentTurn: '',
      winner: '',
      yourFleet: null,
      enemyFleet: null,
      readyPlayers: null,
    } as unknown as Snapshot

    const fixed = normalizeSnapshot(broken)

    expect(fixed.own.ships).toEqual([])
    expect(fixed.enemy.shots).toEqual([])
    expect(fixed.players).toEqual([])
    expect(fixed.yourFleet).toEqual([])
    expect(fixed.enemyFleet).toEqual([])
    expect(fixed.readyPlayers).toEqual([])
  })

  it('keeps a healthy snapshot unchanged', () => {
    const healthy: Snapshot = {
      phase: 'playing',
      width: 10,
      height: 10,
      players: [{ id: 'p1', name: 'Ana', color: '#e94f6d', ready: true }],
      own: { width: 10, height: 10, ships: [], sunkShips: [], shots: [] },
      enemy: { width: 10, height: 10, ships: [], sunkShips: [], shots: [] },
      currentTurn: 'p1',
      winner: '',
      yourFleet: [],
      enemyFleet: [],
      readyPlayers: ['p1'],
    }

    const fixed = normalizeSnapshot(healthy)

    expect(fixed.phase).toBe('playing')
    expect(fixed.players).toHaveLength(1)
    expect(fixed.readyPlayers).toEqual(['p1'])
  })
})
