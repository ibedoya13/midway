import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import App from './App.vue'
import BoardGrid from './components/BoardGrid.vue'
import { GUTTER } from './game/renderer'
import type { Snapshot } from './game/types'

const h = vi.hoisted(() => {
  const stateCallbacks: ((state: Snapshot) => void)[] = []
  const welcomeCallbacks: ((id: string, room: string, key?: string) => void)[] = []
  const errorCallbacks: ((message: string) => void)[] = []
  const closeCallbacks: (() => void)[] = []
  const sent: Record<string, unknown>[] = []

  class FakeClient {
    connect = vi.fn((name?: string) => {
      sent.push({ type: 'connect', name })
    })
    createRoom = vi.fn((name?: string) => {
      sent.push({ type: 'create', name })
    })
    joinRoom = vi.fn((name: string | undefined, room: string, key: string) => {
      sent.push({ type: 'join', name, room, key })
    })
    leaveRoom = vi.fn(() => {
      sent.push({ type: 'leave' })
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
    onWelcome = vi.fn((cb: (id: string, room: string, key?: string) => void) => {
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
    save: vi.fn(),
    restore: vi.fn(),
    translate: vi.fn(),
  } as unknown as CanvasRenderingContext2D)
}

function mountApp() {
  stubCanvas()
  return mount(App)
}

/** clickCell dispatches a click on the cell (x, y) of the board at the given index. */
function clickCell(wrapper: ReturnType<typeof mount>, index: number, x: number, y: number): void {
  const canvas = wrapper.findAll('canvas')[index]
  const event = new MouseEvent('click', { bubbles: true })
  // The play area starts after the gutter that holds the coordinates.
  Object.defineProperty(event, 'offsetX', { value: GUTTER + 24 * x + 3 })
  Object.defineProperty(event, 'offsetY', { value: GUTTER + 24 * y + 4 })
  canvas.element.dispatchEvent(event)
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

  it('opens the websocket connection on mount', () => {
    mountApp()
    expect(h.sent).toContainEqual({ type: 'connect', name: undefined })
  })

  it('offers the two ways to enter a room', async () => {
    const wrapper = mountApp()
    expect(wrapper.find('[data-create]').exists()).toBe(true)
    expect(wrapper.find('[data-join-room]').exists()).toBe(true)
  })

  it('creates a room with the typed name', async () => {
    const wrapper = mountApp()
    await wrapper.find('[data-name]').setValue('Ana')
    await wrapper.find('[data-create]').trigger('submit')
    expect(h.sent).toContainEqual({ type: 'create', name: 'Ana' })
  })

  it('joins an existing room with its code and its key', async () => {
    const wrapper = mountApp()
    await wrapper.find('[data-name]').setValue('Beto')
    await wrapper.find('[data-room-code]').setValue('123456')
    await wrapper.find('[data-room-key]').setValue('a'.repeat(32))
    await wrapper.find('[data-join-room]').trigger('submit')
    expect(h.sent).toContainEqual({
      type: 'join',
      name: 'Beto',
      room: '123456',
      key: 'a'.repeat(32),
    })
  })

  it('does not send a join without a valid key', async () => {
    const wrapper = mountApp()
    await wrapper.find('[data-name]').setValue('Beto')
    await wrapper.find('[data-room-code]').setValue('123456')
    await wrapper.find('[data-join-room]').trigger('submit')
    expect(h.sent.filter((m) => m.type === 'join')).toHaveLength(0)
    expect(wrapper.find('[data-error]').text()).toContain('clave')
  })

  it('does not send a join with a malformed code', async () => {
    const wrapper = mountApp()
    await wrapper.find('[data-name]').setValue('Beto')
    await wrapper.find('[data-room-code]').setValue('12')
    await wrapper.find('[data-join-room]').trigger('submit')
    expect(h.sent.filter((m) => m.type === 'join')).toHaveLength(0)
    expect(wrapper.find('[data-error]').text()).toContain('seis dígitos')
  })

  it('shows the room code and the invite link once in a room', async () => {
    const wrapper = mountApp()
    h.welcomeCallbacks.forEach((cb) => cb('p1', '123456', 'a'.repeat(32)))
    h.stateCallbacks.forEach((cb) => cb(snapshot()))
    await flushPromises()

    expect(wrapper.find('[data-room-code-display]').text()).toContain('123456')
    expect(wrapper.find('[data-invite-link]').text()).toContain('room=123456')
    expect(wrapper.find('[data-invite-link]').text()).toContain(`key=${'a'.repeat(32)}`)
  })

  it('keeps the invite link for a player that joined with a key', async () => {
    const wrapper = mountApp()
    await wrapper.find('[data-name]').setValue('Beto')
    await wrapper.find('[data-room-code]').setValue('123456')
    await wrapper.find('[data-room-key]').setValue('a'.repeat(32))
    await wrapper.find('[data-join-room]').trigger('submit')
    // The welcome of a player that joined carries no key: the one it used
    // has to be kept.
    h.welcomeCallbacks.forEach((cb) => cb('p1', '123456'))
    h.stateCallbacks.forEach((cb) => cb(snapshot()))
    await flushPromises()

    expect(wrapper.find('[data-invite-link]').text()).toContain(`key=${'a'.repeat(32)}`)
  })

  it('warns when the opponent is disconnected', async () => {
    const wrapper = mountApp()
    h.welcomeCallbacks.forEach((cb) => cb('p1', '123456'))
    const state = snapshot()
    state.players = [
      { id: 'p1', name: 'Ana', color: '#e94f6d', ready: true },
      { id: 'p2', name: 'Beto', color: '#4f8ef7', ready: true, disconnected: true },
    ]
    h.stateCallbacks.forEach((cb) => cb(state))
    await flushPromises()

    expect(wrapper.find('[data-opponent-away]').exists()).toBe(true)
  })

  it('goes back to the lobby when leaving the room', async () => {
    const wrapper = mountApp()
    h.welcomeCallbacks.forEach((cb) => cb('p1', '123456'))
    h.stateCallbacks.forEach((cb) => cb(snapshot()))
    await flushPromises()

    await wrapper.find('[data-leave]').trigger('click')
    await flushPromises()

    expect(h.sent).toContainEqual({ type: 'leave' })
    expect(wrapper.find('[data-join]').exists()).toBe(true)
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
    Object.defineProperty(event, 'offsetX', { value: GUTTER + 24 * 2 + 3 })
    Object.defineProperty(event, 'offsetY', { value: GUTTER + 24 * 4 + 4 })
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
    Object.defineProperty(event, 'offsetX', { value: GUTTER + 24 * 1 + 1 })
    Object.defineProperty(event, 'offsetY', { value: GUTTER + 24 * 2 + 2 })
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
    Object.defineProperty(event, 'offsetX', { value: GUTTER + 24 * 1 + 1 })
    Object.defineProperty(event, 'offsetY', { value: GUTTER + 24 * 2 + 2 })
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

  it('clears the selected ship once it is placed', async () => {
    const wrapper = mountApp()
    h.welcomeCallbacks.forEach((cb) => cb('p1', 'room-1'))
    h.stateCallbacks.forEach((cb) => cb(snapshot()))
    await flushPromises()

    await wrapper.find('[data-ship="carrier"]').trigger('click')

    // The first click places the carrier.
    clickCell(wrapper, 0, 2, 4)
    await flushPromises()
    expect(h.sent.filter((m) => m.type === 'place')).toHaveLength(1)

    // The server confirms the placement: the ship now appears in own.ships.
    h.stateCallbacks.forEach((cb) =>
      cb(
        snapshot({
          own: {
            width: 10,
            height: 10,
            sunkShips: [],
            shots: [],
            ships: [
              {
                kind: 'carrier',
                name: 'Portaaviones',
                size: 5,
                x: 2,
                y: 4,
                orientation: 'horizontal',
              },
            ],
          },
        }),
      ),
    )
    await flushPromises()

    // Clicking the board again must not try to place the carrier a second
    // time: the server would answer "el portaaviones ya fue colocado".
    clickCell(wrapper, 0, 2, 4)
    await flushPromises()
    expect(h.sent.filter((m) => m.type === 'place')).toHaveLength(1)

    // Prove the click still reaches the board: placing another ship works.
    await wrapper.find('[data-ship="cruiser"]').trigger('click')
    clickCell(wrapper, 0, 6, 6)
    await flushPromises()

    const places = h.sent.filter((m) => m.type === 'place')
    expect(places).toHaveLength(2)
    expect(places[1]).toEqual({
      type: 'place',
      kind: 'cruiser',
      x: 6,
      y: 6,
      orientation: 'horizontal',
    })
  })

  it('paints the enemy board with the opponent color', async () => {
    const wrapper = mountApp()
    // I am the second player (blue), so the enemy board must be red.
    h.welcomeCallbacks.forEach((cb) => cb('p2', 'room-1'))
    h.stateCallbacks.forEach((cb) => cb(snapshot()))
    await flushPromises()

    const grids = wrapper.findAllComponents(BoardGrid)
    expect(grids).toHaveLength(2)
    expect(grids[0].props('playerColor')).toBe('#4f8ef7')
    expect(grids[1].props('playerColor')).toBe('#e94f6d')
  })

  it('paints the enemy board with the opponent color for the first player', async () => {
    const wrapper = mountApp()
    // I am the first player (red), so the enemy board must be blue.
    h.welcomeCallbacks.forEach((cb) => cb('p1', 'room-1'))
    h.stateCallbacks.forEach((cb) => cb(snapshot()))
    await flushPromises()

    const grids = wrapper.findAllComponents(BoardGrid)
    expect(grids[0].props('playerColor')).toBe('#e94f6d')
    expect(grids[1].props('playerColor')).toBe('#4f8ef7')
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
