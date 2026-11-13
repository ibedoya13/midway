import { normalizeSnapshot } from './normalize'
import type { ClientMessage, Orientation, ServerMessage, ShipKind, Snapshot } from './types'

export interface GameClientOptions {
  socketFactory?: (url: string) => WebSocket
}

/** MAX_RECONNECT_ATTEMPTS is how many times the client tries to reconnect. */
const MAX_RECONNECT_ATTEMPTS = 8
/** RECONNECT_BASE_DELAY is the delay of the first reconnection attempt. */
const RECONNECT_BASE_DELAY = 1000
/** RECONNECT_MAX_DELAY caps the delay between attempts. */
const RECONNECT_MAX_DELAY = 15000

type StateListener = (state: Snapshot) => void
type ErrorListener = (message: string) => void
type WelcomeListener = (
  playerId: string,
  room: string,
  key?: string,
  resumeToken?: string,
) => void
type CloseListener = () => void

/**
 * GameClient wraps the WebSocket connection with the game protocol.
 *
 * Messages sent before the socket is open are queued and flushed on open.
 */
export class GameClient {
  private readonly url: string
  private readonly socketFactory: (url: string) => WebSocket
  private socket: WebSocket | null = null
  private queue: ClientMessage[] = []

  /** resumeToken identifies the player across a reconnection. */
  resumeToken = ''
  /** roomCode is the code of the room the player is in. */
  private roomCode = ''
  /** playerName is the name used to rejoin the room. */
  private playerName = ''
  /** roomKey is the secret key of the room, used to rejoin it. */
  private roomKey = ''
  /** retries counts the reconnection attempts. */
  private retries = 0
  /** reconnectTimer holds the pending reconnection. */
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  /** manualClose is true when the client closed the connection on purpose. */
  private manualClose = false

  private stateListeners: StateListener[] = []
  private errorListeners: ErrorListener[] = []
  private welcomeListeners: WelcomeListener[] = []
  private closeListeners: CloseListener[] = []

  constructor(url: string, options: GameClientOptions = {}) {
    this.url = url
    this.socketFactory = options.socketFactory ?? ((target) => new WebSocket(target))
  }

  connect(): void {
    this.disconnect()
    this.manualClose = false
    this.openSocket()
  }

  /** openSocket creates the socket and wires its handlers. */
  private openSocket(): void {
    const socket = this.socketFactory(this.url)
    this.socket = socket

    socket.onopen = () => {
      this.flush()
    }
    socket.onmessage = (event: MessageEvent) => {
      this.handle(event.data)
    }
    socket.onclose = () => {
      this.closeListeners.forEach((listener) => listener())
      this.scheduleReconnect()
    }
    socket.onerror = () => {
      this.errorListeners.forEach((listener) => listener('error de conexión'))
    }
  }

  /**
   * scheduleReconnect tries to open the connection again after it was lost.
   *
   * Cloud Run closes the WebSocket after 60 minutes, and a network hiccup
   * does the same. Without this the player loses the match. The attempt
   * stops after MAX_RECONNECT_ATTEMPTS.
   */
  private scheduleReconnect(): void {
    if (this.manualClose) return
    if (!this.roomCode) return
    if (this.retries >= MAX_RECONNECT_ATTEMPTS) {
      this.errorListeners.forEach((listener) =>
        listener('se perdió la conexión con el servidor'),
      )
      return
    }

    this.retries += 1
    const delay = Math.min(RECONNECT_BASE_DELAY * 2 ** (this.retries - 1), RECONNECT_MAX_DELAY)
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.openSocket()
      this.rejoin()
    }, delay)
  }

  /** rejoin sends the join message again after a reconnection. */
  private rejoin(): void {
    if (!this.roomCode) return
    this.send({
      type: 'join',
      name: this.playerName || undefined,
      room: this.roomCode,
      key: this.roomKey || undefined,
      resumeToken: this.resumeToken || undefined,
    })
  }

  /** createRoom asks the server for a new room and joins it. */
  createRoom(name?: string): void {
    this.playerName = name ?? ''
    this.send({ type: 'create', name })
  }

  /** joinRoom joins an existing room identified by its six-digit code and
   * its secret key. */
  joinRoom(name: string | undefined, code: string, key: string): void {
    this.playerName = name ?? ''
    this.roomCode = code
    this.roomKey = key
    this.send({ type: 'join', name, room: code, key })
  }

  /** leaveRoom leaves the current room and returns to the lobby. */
  leaveRoom(): void {
    // The player gives up its seat: it must not reconnect.
    this.roomCode = ''
    this.roomKey = ''
    this.resumeToken = ''
    this.retries = 0
    this.send({ type: 'leave' })
  }

  disconnect(): void {
    // The connection is closed on purpose: it must not reconnect.
    this.manualClose = true
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    const socket = this.socket
    this.socket = null
    if (socket) {
      socket.onopen = null
      socket.onmessage = null
      socket.onerror = null
      if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) {
        socket.close()
      }
      socket.onclose = null
    }
  }

  place(kind: ShipKind, x: number, y: number, orientation: Orientation): void {
    this.send({ type: 'place', kind, x, y, orientation })
  }

  removeShip(kind: ShipKind): void {
    this.send({ type: 'remove', kind })
  }

  ready(): void {
    this.send({ type: 'ready' })
  }

  shoot(x: number, y: number): void {
    this.send({ type: 'shoot', x, y })
  }

  start(): void {
    this.send({ type: 'start' })
  }

  ping(): void {
    this.send({ type: 'ping' })
  }

  onState(cb: StateListener): void {
    this.stateListeners.push(cb)
  }

  onError(cb: ErrorListener): void {
    this.errorListeners.push(cb)
  }

  onWelcome(cb: WelcomeListener): void {
    this.welcomeListeners.push(cb)
  }

  onClose(cb: CloseListener): void {
    this.closeListeners.push(cb)
  }

  private send(message: ClientMessage): void {
    if (this.socket && this.socket.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify(message))
      return
    }
    this.queue.push(message)
  }

  private flush(): void {
    const pending = this.queue
    this.queue = []
    pending.forEach((message) => {
      if (this.socket && this.socket.readyState === WebSocket.OPEN) {
        this.socket.send(JSON.stringify(message))
      }
    })
  }

  private handle(raw: unknown): void {
    let message: ServerMessage
    try {
      message = JSON.parse(String(raw)) as ServerMessage
    } catch {
      this.errorListeners.forEach((listener) => listener('mensaje inválido del servidor'))
      return
    }
    if (!message || typeof message.type !== 'string') return

    switch (message.type) {
      case 'welcome':
        this.resumeToken = message.resumeToken ?? this.resumeToken
        if (message.room) {
          this.roomCode = message.room
        }
        // The connection is alive again: the reconnection attempts restart.
        this.retries = 0
        this.welcomeListeners.forEach((listener) =>
          listener(
            message.playerId ?? '',
            message.room ?? '',
            message.key,
            message.resumeToken,
          ),
        )
        break
      case 'state':
        if (message.state) {
          // A malformed payload must not break the UI: normalize it first.
          const state = normalizeSnapshot(message.state as Snapshot)
          this.stateListeners.forEach((listener) => listener(state))
        }
        break
      case 'error':
        this.errorListeners.forEach((listener) => listener(message.message ?? 'error desconocido'))
        break
      case 'pong':
        break
    }
  }
}
