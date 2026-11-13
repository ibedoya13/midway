import { beforeEach, describe, expect, it, vi } from 'vitest'

import { GameClient } from './client'
import type { ServerMessage, Snapshot } from './types'

class FakeSocket {
  static instances: FakeSocket[] = []

  url: string
  readyState = 0
  sent: string[] = []
  onopen: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null

  constructor(url: string) {
    this.url = url
    FakeSocket.instances.push(this)
  }

  open(): void {
    this.readyState = 1
    this.onopen?.()
  }

  send(data: string): void {
    this.sent.push(data)
  }

  close(): void {
    this.readyState = 3
    this.onclose?.()
  }

  receive(message: ServerMessage): void {
    this.onmessage?.({ data: JSON.stringify(message) })
  }

  parsed(): Record<string, unknown>[] {
    return this.sent.map((raw) => JSON.parse(raw) as Record<string, unknown>)
  }
}

function makeSnapshot(phase: Snapshot['phase'] = 'setup'): Snapshot {
  return {
    phase,
    width: 10,
    height: 10,
    players: [],
    own: { width: 10, height: 10, ships: [], sunkShips: [], shots: [] },
    enemy: { width: 10, height: 10, ships: [], sunkShips: [], shots: [] },
    currentTurn: '',
    winner: '',
    yourFleet: [],
    enemyFleet: [],
    readyPlayers: [],
  }
}

describe('GameClient', () => {
  beforeEach(() => {
    FakeSocket.instances = []
  })

  it('creates the socket with the given url when connecting', () => {
    const client = new GameClient('ws://localhost:8080/ws', {
      socketFactory: (url) => new FakeSocket(url) as unknown as WebSocket,
    })
    client.connect()
    expect(FakeSocket.instances).toHaveLength(1)
    expect(FakeSocket.instances[0].url).toBe('ws://localhost:8080/ws')
  })

  it('queues messages until the socket is open and flushes them on open', () => {
    const client = new GameClient('ws://x', {
      socketFactory: (url) => new FakeSocket(url) as unknown as WebSocket,
    })
    client.connect()
    client.place('carrier', 0, 0, 'horizontal')
    client.ready()
    const socket = FakeSocket.instances[0]
    expect(socket.sent).toHaveLength(0)
    socket.open()
    expect(socket.parsed()).toEqual([
      { type: 'place', kind: 'carrier', x: 0, y: 0, orientation: 'horizontal' },
      { type: 'ready' },
    ])
  })

  it('sends a create message with the name', () => {
    const client = new GameClient('ws://x', {
      socketFactory: (url) => new FakeSocket(url) as unknown as WebSocket,
    })
    client.connect()
    FakeSocket.instances[0].open()
    client.createRoom('Ana')

    expect(FakeSocket.instances[0].parsed()).toContainEqual({ type: 'create', name: 'Ana' })
  })

  it('sends a join message with the room code and the secret key', () => {
    const client = new GameClient('ws://x', {
      socketFactory: (url) => new FakeSocket(url) as unknown as WebSocket,
    })
    client.connect()
    FakeSocket.instances[0].open()
    client.joinRoom('Beto', '123456', 'a'.repeat(32))

    expect(FakeSocket.instances[0].parsed()).toContainEqual({
      type: 'join',
      name: 'Beto',
      room: '123456',
      key: 'a'.repeat(32),
    })
  })

  it('sends a leave message', () => {
    const client = new GameClient('ws://x', {
      socketFactory: (url) => new FakeSocket(url) as unknown as WebSocket,
    })
    client.connect()
    FakeSocket.instances[0].open()
    client.leaveRoom()

    expect(FakeSocket.instances[0].parsed()).toContainEqual({ type: 'leave' })
  })

  it('sends each action with the right payload once open', () => {
    const client = new GameClient('ws://x', {
      socketFactory: (url) => new FakeSocket(url) as unknown as WebSocket,
    })
    client.connect()
    FakeSocket.instances[0].open()
    const socket = FakeSocket.instances[0]
    client.place('cruiser', 3, 4, 'vertical')
    client.removeShip('cruiser')
    client.shoot(5, 6)
    client.start()
    client.ping()
    expect(socket.parsed()).toEqual([
      { type: 'place', kind: 'cruiser', x: 3, y: 4, orientation: 'vertical' },
      { type: 'remove', kind: 'cruiser' },
      { type: 'shoot', x: 5, y: 6 },
      { type: 'start' },
      { type: 'ping' },
    ])
  })

  it('dispatches state, error and welcome messages to their callbacks', () => {
    const client = new GameClient('ws://x', {
      socketFactory: (url) => new FakeSocket(url) as unknown as WebSocket,
    })
    const onState = vi.fn()
    const onError = vi.fn()
    const onWelcome = vi.fn()
    client.onState(onState)
    client.onError(onError)
    client.onWelcome(onWelcome)
    client.connect()
    const socket = FakeSocket.instances[0]
    socket.open()
    const snapshot = makeSnapshot('playing')
    socket.receive({ type: 'welcome', playerId: 'p1', room: '123456', key: 'a'.repeat(32) })
    socket.receive({ type: 'state', state: snapshot })
    socket.receive({ type: 'error', message: 'boom' })
    expect(onWelcome).toHaveBeenCalledWith('p1', '123456', 'a'.repeat(32))
    expect(onState).toHaveBeenCalledWith(snapshot)
    expect(onError).toHaveBeenCalledWith('boom')
  })

  it('calls the close callback when the socket closes', () => {
    const client = new GameClient('ws://x', {
      socketFactory: (url) => new FakeSocket(url) as unknown as WebSocket,
    })
    const onClose = vi.fn()
    client.onClose(onClose)
    client.connect()
    const socket = FakeSocket.instances[0]
    socket.open()
    client.disconnect()
    expect(onClose).toHaveBeenCalled()
    expect(socket.readyState).toBe(3)
  })

  it('ignores malformed payloads without throwing', () => {
    const client = new GameClient('ws://x', {
      socketFactory: (url) => new FakeSocket(url) as unknown as WebSocket,
    })
    const onError = vi.fn()
    client.onError(onError)
    client.connect()
    const socket = FakeSocket.instances[0]
    socket.open()
    expect(() => socket.onmessage?.({ data: 'not json' })).not.toThrow()
  })
})
