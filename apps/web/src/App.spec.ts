import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import App from './App.vue'
import type { Snapshot } from './game/types'

const h = vi.hoisted(() => {
  const stateCallbacks: ((state: Snapshot) => void)[] = []
  const welcomeCallbacks: ((id: string, room: string) => void)[] = []
  const errorCallbacks: ((message: string) => void)[] = []
  const closeCallbacks: (() => void)[] = []
  const sent: Record<string, unknown>[] = []

  class FakeClient {
    connect = vi.fn((name?: string) => {
      sent.push({ type: 'connect', name })
    })
    disconnect = vi.fn()
    place = vi.fn((kind: string, x: number, y: number, orientation: string) => {
      sent.push({ type: 'place', kind, x, y, orientation })
    })
    removeShip = vi.fn((kind: string) => {
      sent.push({ type: 'remove', kind })
    })
    ready = vi.fn(() => {
      sent.push({ type: 'ready' })
    })
    shoot = vi.fn((x: number, y: number) => {
      sent.push({ type: 'shoot', x, y })
    })
    start = vi.fn(() => {
      sent.push({ type: 'start' })
    })
    ping = vi.fn()
    onState = vi.fn((cb: (state: Snapshot) => void) => {
      stateCallbacks.push(cb)
    })
    onError = vi.fn((cb: (message: string) => void) => {
      errorCallbacks.push(cb)
    })
    onWelcome = vi.fn((cb: (id: string, room: string) => void) => {
      welcomeCallbacks.push(cb)
    })
    onClose = vi.fn((cb: () => void) => {
      closeCallbacks.push(cb)
    })
  }

  return {
    FakeClient,
    stateCallbacks,
    welcomeCallbacks,
    errorCallbacks,
    closeCallbacks,
    sent,
  }
})

vi.mock('./game/client', () => ({
  GameClient: h.FakeClient,
}))

function snapshot(overrides: Partial<Snapshot> = {}): Snapshot {
  return {
    phase: 'setup',
    width: 10,
    height: 10,
    players: [
      { id: 'p1', name: 'Ana', color: '#e94f6d', ready: false },
      { id: 'p2', name: 'Beto', color: '#4f8ef7', ready: false },
    ],
    own: { width: 10, height: 10, ships: [], sunkShips: [], shots: [] },
    enemy: { width: 10, height: 10, ships: [], sunkShips: [], shots: [] },
    currentTurn: '',
    winner: '',
    yourFleet: [
      { kind: 'carrier', name: 'Portaaviones', size: 5 },
      { kind: 'battleship', name: 'Acorazado', size: 4 },
      { kind: 'cruiser', name: 'Crucero', size: 3 },
      { kind: 'submarine', name: 'Submarino', size: 3 },
      { kind: 'destroyer', name: 'Destructor', size: 2 },
    ],
    enemyFleet: [],
    readyPlayers: [],
    ...overrides,
  }
}

function stubCanvas(): void {
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
    imageSmoothingEnabled: true,
    fillStyle: '#000000',
    fillRect: vi.fn(),
  } as unknown as CanvasRenderingContext2D)
}

function mountApp() {
  stubCanvas()
  return mount(App)
}

describe('App', () => {
  beforeEach(() => {
    h.stateCallbacks.length = 0
    h.welcomeCallbacks.length = 0
    h.errorCallbacks.length = 0
    h.closeCallbacks.length = 0
    h.sent.length = 0
  })

  it('shows the connection screen first', () => {
    const wrapper = mountApp()
    expect(wrapper.find('[data-join]').exists()).toBe(true)
    expect(wrapper.find('canvas').exists()).toBe(false)
  })

  it('connects with the typed name', async () => {
    const wrapper = mountApp()
    await wrapper.find('[data-name]').setValue('Ana')
    await wrapper.find('[data-enter]').trigger('submit')
    expect(h.sent).toContainEqual({ type: 'connect', name: 'Ana' })
  })

  it('shows an error message from the server', async () => {
    const wrapper = mountApp()
    h.errorCallbacks.forEach((cb) => cb('no se puede entrar'))
    await flushPromises()
    expect(wrapper.find('[data-error]').text()).toContain('no se puede entrar')
  })

  it('renders both boards and the fleet panel during setup', async () => {
    const wrapper = mountApp()
    h.welcomeCallbacks.forEach((cb) => cb('p1', 'room-1'))
    h.stateCallbacks.forEach((cb) => cb(snapshot()))
    await flushPromises()
    expect(wrapper.findAll('canvas')).toHaveLength(2)
    expect(wrapper.find('[data-fleet]').exists()).toBe(true)
  })

  it('places the selected ship when clicking the own board', async () => {
    const wrapper = mountApp()
    h.welcomeCallbacks.forEach((cb) => cb('p1', 'room-1'))
    h.stateCallbacks.forEach((cb) => cb(snapshot()))
    await flushPromises()
    await wrapper.find('[data-ship="carrier"]').trigger('click')
    const canvas = wrapper.findAll('canvas')[0]
    const event = new MouseEvent('click', { bubbles: true })
    Object.defineProperty(event, 'offsetX', { value: 24 * 2 + 3 })
    Object.defineProperty(event, 'offsetY', { value: 24 * 4 + 4 })
    canvas.element.dispatchEvent(event)
    await flushPromises()
    expect(h.sent).toContainEqual({
      type: 'place',
      kind: 'carrier',
      x: 2,
      y: 4,
      orientation: 'horizontal',
    })
  })

  it('shoots at the enemy board only on your turn', async () => {
    const wrapper = mountApp()
    h.welcomeCallbacks.forEach((cb) => cb('p1', 'room-1'))
    h.stateCallbacks.forEach((cb) =>
      cb(snapshot({ phase: 'playing', currentTurn: 'p2' })),
    )
    await flushPromises()
    const enemy = wrapper.findAll('canvas')[1]
    const event = new MouseEvent('click', { bubbles: true })
    Object.defineProperty(event, 'offsetX', { value: 24 * 1 + 1 })
    Object.defineProperty(event, 'offsetY', { value: 24 * 2 + 2 })
    enemy.element.dispatchEvent(event)
    await flushPromises()
    expect(h.sent.filter((m) => m.type === 'shoot')).toHaveLength(0)
  })

  it('shoots at the enemy board when it is your turn', async () => {
    const wrapper = mountApp()
    h.welcomeCallbacks.forEach((cb) => cb('p1', 'room-1'))
    h.stateCallbacks.forEach((cb) =>
      cb(snapshot({ phase: 'playing', currentTurn: 'p1' })),
    )
    await flushPromises()
    const enemy = wrapper.findAll('canvas')[1]
    const event = new MouseEvent('click', { bubbles: true })
    Object.defineProperty(event, 'offsetX', { value: 24 * 1 + 1 })
    Object.defineProperty(event, 'offsetY', { value: 24 * 2 + 2 })
    enemy.element.dispatchEvent(event)
    await flushPromises()
    expect(h.sent).toContainEqual({ type: 'shoot', x: 1, y: 2 })
  })

  it('shoots with the keyboard using A-J and 1-10', async () => {
    mountApp()
    h.welcomeCallbacks.forEach((cb) => cb('p1', 'room-1'))
    h.stateCallbacks.forEach((cb) =>
      cb(snapshot({ phase: 'playing', currentTurn: 'p1' })),
    )
    await flushPromises()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'c' }))
    window.dispatchEvent(new KeyboardEvent('keydown', { key: '5' }))
    await flushPromises()
    expect(h.sent).toContainEqual({ type: 'shoot', x: 2, y: 4 })
  })

  it('shows the game over screen with the winner', async () => {
    const wrapper = mountApp()
    h.welcomeCallbacks.forEach((cb) => cb('p1', 'room-1'))
    h.stateCallbacks.forEach((cb) =>
      cb(snapshot({ phase: 'gameover', winner: 'p1' })),
    )
    await flushPromises()
    expect(wrapper.find('[data-gameover]').text()).toContain('Ana')
    expect(wrapper.find('[data-restart]').exists()).toBe(true)
  })

  it('restarts the game from the game over screen', async () => {
    const wrapper = mountApp()
    h.welcomeCallbacks.forEach((cb) => cb('p1', 'room-1'))
    h.stateCallbacks.forEach((cb) =>
      cb(snapshot({ phase: 'gameover', winner: 'p1' })),
    )
    await flushPromises()
    await wrapper.find('[data-restart]').trigger('click')
    expect(h.sent).toContainEqual({ type: 'start' })
  })

  it('sends ready when the fleet panel is ready', async () => {
    const wrapper = mountApp()
    h.welcomeCallbacks.forEach((cb) => cb('p1', 'room-1'))
    h.stateCallbacks.forEach((cb) =>
      cb(
        snapshot({
          own: {
            width: 10,
            height: 10,
            sunkShips: [],
            shots: [],
            ships: [
              { kind: 'carrier', name: 'Portaaviones', size: 5, x: 0, y: 0, orientation: 'horizontal' },
              { kind: 'battleship', name: 'Acorazado', size: 4, x: 0, y: 1, orientation: 'horizontal' },
              { kind: 'cruiser', name: 'Crucero', size: 3, x: 0, y: 2, orientation: 'horizontal' },
              { kind: 'submarine', name: 'Submarino', size: 3, x: 0, y: 3, orientation: 'horizontal' },
              { kind: 'destroyer', name: 'Destructor', size: 2, x: 0, y: 4, orientation: 'horizontal' },
            ],
          },
        }),
      ),
    )
    await flushPromises()
    await wrapper.find('[data-ready]').trigger('click')
    expect(h.sent).toContainEqual({ type: 'ready' })
  })
})
