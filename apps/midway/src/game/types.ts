/**
 * Contrato de tipos compartido con el servidor Go (apps/server/internal/ws).
 * Cualquier cambio aquí debe reflejarse en protocol.go y en el README.
 */

export type GamePhase = 'setup' | 'playing' | 'gameover'

export type ShipKind = 'carrier' | 'battleship' | 'cruiser' | 'submarine' | 'destroyer'

export type Orientation = 'horizontal' | 'vertical'

export type ShotResult = 'water' | 'hit' | 'sunk'

export interface ShipSpec {
  kind: ShipKind
  name: string
  size: number
}

export interface ShipView extends ShipSpec {
  x: number
  y: number
  orientation: Orientation
}

export interface ShotView {
  x: number
  y: number
  result: ShotResult
}

export interface BoardView {
  width: number
  height: number
  /** Barcos visibles (solo en el tablero propio). */
  ships: ShipView[]
  /** Barcos ya hundidos (propios y enemigos). */
  sunkShips: ShipView[]
  /** Disparos registrados sobre el tablero. */
  shots: ShotView[]
}


/** PlayerInfo describes a player of the match. */
export interface PlayerInfo {
  id: string
  name: string
  color: string
  ready: boolean
  /** True while the player is away and waiting to come back. */
  disconnected?: boolean
}

export interface Snapshot {
  phase: GamePhase
  width: number
  height: number
  players: PlayerInfo[]
  /** Tu propio tablero: con tus barcos y los disparos que recibiste. */
  own: BoardView
  /** Tablero enemigo: sin barcos, solo disparos y hundimientos. */
  enemy: BoardView
  currentTurn: string
  winner: string
  yourFleet: ShipSpec[]
  enemyFleet: ShipSpec[]
  readyPlayers: string[]
}

export interface ClientMessage {
  type:
    | 'create'
    | 'join'
    | 'leave'
    | 'place'
    | 'remove'
    | 'ready'
    | 'shoot'
    | 'start'
    | 'ping'
  name?: string
  /** Six-digit code of the room to join. */
  room?: string
  /** Secret key of the room: the credential that admits a player. */
  key?: string
  /** Token that identifies the player across a reconnection. */
  resumeToken?: string
  kind?: ShipKind
  x?: number
  y?: number
  orientation?: Orientation
}

export interface ServerMessage {
  type: 'welcome' | 'state' | 'error' | 'pong' | 'left'
  playerId?: string
  /** Six-digit code of the room the player joined. */
  room?: string
  /** Secret key of the room, sent only to the player that created it. */
  key?: string
  /** Token that identifies the player across a reconnection. */
  resumeToken?: string
  message?: string
  state?: Snapshot
}
