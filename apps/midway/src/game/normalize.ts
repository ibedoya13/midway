import type { BoardView, PlayerInfo, ShipSpec, ShipView, ShotView, Snapshot } from './types'

/**
 * Defense in depth against malformed payloads.
 *
 * The Go server serializes empty slices as `null` instead of `[]` unless it is
 * careful, and a `null` list breaks every `forEach` in the renderer. These
 * helpers make the client resilient: a malformed field becomes an empty list
 * instead of a crash that takes the whole UI down.
 */

/** asArray returns the value when it is an array, and an empty array otherwise. */
function asArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}

/**
 * normalizeBoard repairs a board view coming from the server.
 */
export function normalizeBoard(board: BoardView | null | undefined): BoardView {
  const source = (board ?? {}) as Partial<BoardView>
  return {
    width: typeof source.width === 'number' ? source.width : 10,
    height: typeof source.height === 'number' ? source.height : 10,
    ships: asArray<ShipView>(source.ships),
    sunkShips: asArray<ShipView>(source.sunkShips),
    shots: asArray<ShotView>(source.shots),
  }
}

/**
 * normalizeSnapshot repairs a full snapshot coming from the server.
 */
export function normalizeSnapshot(snapshot: Snapshot | null | undefined): Snapshot {
  const source = (snapshot ?? {}) as Partial<Snapshot>
  return {
    phase: source.phase ?? 'setup',
    width: typeof source.width === 'number' ? source.width : 10,
    height: typeof source.height === 'number' ? source.height : 10,
    players: asArray<PlayerInfo>(source.players),
    own: normalizeBoard(source.own),
    enemy: normalizeBoard(source.enemy),
    currentTurn: source.currentTurn ?? '',
    winner: source.winner ?? '',
    yourFleet: asArray<ShipSpec>(source.yourFleet),
    enemyFleet: asArray<ShipSpec>(source.enemyFleet),
    readyPlayers: asArray<string>(source.readyPlayers),
  }
}
