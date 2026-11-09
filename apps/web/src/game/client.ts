import type { ClientMessage, Orientation, ServerMessage, ShipKind, Snapshot } from './types'

export interface GameClientOptions {
  socketFactory?: (url: string) => WebSocket
}

type StateListener = (state: Snapshot) => void
type ErrorListener = (message: string) => void
type WelcomeListener = (playerId: string, room: string) => void
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

  private stateListeners: StateListener[] = []
  private errorListeners: ErrorListener[] = []
  private welcomeListeners: WelcomeListener[] = []
  private closeListeners: CloseListener[] = []

  constructor(url: string, options: GameClientOptions = {}) {
    this.url = url
    this.socketFactory = options.socketFactory ?? ((target) => new WebSocket(target))
  }

  connect(name?: string): void {
    this.disconnect()
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
    }
    socket.onerror = () => {
      this.errorListeners.forEach((listener) => listener('error de conexión'))
    }

    this.send({ type: 'join', name })
  }

  disconnect(): void {
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
        this.welcomeListeners.forEach((listener) =>
          listener(message.playerId ?? '', message.room ?? ''),
        )
        break
      case 'state':
        if (message.state) {
          this.stateListeners.forEach((listener) => listener(message.state as Snapshot))
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
