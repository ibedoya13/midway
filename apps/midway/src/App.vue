<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import BoardGrid from './components/BoardGrid.vue'
import FleetPanel from './components/FleetPanel.vue'
import HudPanel from './components/HudPanel.vue'
import { GameClient } from './game/client'
import { COLUMN_LABELS } from './game/board'
import {
  buildInviteLink,
  isValidRoomCode,
  isValidRoomKey,
  roomParamsFromUrl,
} from './game/room'
import { resolveWebSocketUrl } from './game/url'
import type { Orientation, ShipKind, Snapshot } from './game/types'

// La URL del backend se deriva del host de la página: así, un teléfono en la
// misma red se conecta a la IP de tu máquina y no a su propio localhost.
const WS_URL = resolveWebSocketUrl(
  window.location.host,
  import.meta.env.VITE_WS_URL,
)

const client = new GameClient(WS_URL)

const snapshot = ref<Snapshot | null>(null)
const playerId = ref('')
// The room code of the match the player joined.
const room = ref('')
const playerName = ref('')
// The code typed to join an existing room.
const roomCodeInput = ref(roomParamsFromUrl(window.location.href).code)
// The secret key of the room, read from the invite link.
const roomKeyInput = ref(roomParamsFromUrl(window.location.href).key)
// The secret key of the room the player created, or the one it joined with.
const roomKey = ref('')
// True while the client is trying to reconnect after losing the connection.
const reconnecting = ref(false)
const error = ref('')
const orientation = ref<Orientation>('horizontal')
const selectedKind = ref<ShipKind | null>(null)

const keyboardX = ref<number | null>(null)
const keyboardY = ref<number | null>(null)

const phase = computed(() => snapshot.value?.phase ?? null)
const connected = computed(() => snapshot.value !== null)

const players = computed(() => snapshot.value?.players ?? [])
const ownBoard = computed(
  () =>
    snapshot.value?.own ?? {
      width: 10,
      height: 10,
      ships: [],
      sunkShips: [],
      shots: [],
    },
)
const enemyBoard = computed(
  () =>
    snapshot.value?.enemy ?? {
      width: 10,
      height: 10,
      ships: [],
      sunkShips: [],
      shots: [],
    },
)
const myFleet = computed(() => snapshot.value?.yourFleet ?? [])
const placedShips = computed(() => ownBoard.value.ships)
const placedKinds = computed(() => new Set(placedShips.value.map((ship) => ship.kind)))
const canReady = computed(() => myFleet.value.length > 0 && placedShips.value.length === myFleet.value.length)
const isMyTurn = computed(() => snapshot.value?.currentTurn === playerId.value)
const winnerName = computed(() => {
  const winner = players.value.find((player) => player.id === snapshot.value?.winner)
  return winner ? winner.name : ''
})
const myColor = computed(
  () => players.value.find((player) => player.id === playerId.value)?.color ?? '#e94f6d',
)
// The enemy board uses the color of the opponent, so each player sees their
// own sea in their own color and the rival sea in the rival color.
const enemyColor = computed(
  () =>
    players.value.find((player) => player.id !== playerId.value)?.color ?? '#4f8ef7',
)

/** createRoom asks the server for a new room. */
function createRoom(): void {
  error.value = ''
  client.createRoom(playerName.value.trim() || undefined)
}

/** joinRoom enters an existing room with its six-digit code and its
 * secret key. */
function joinRoom(): void {
  const code = roomCodeInput.value.trim()
  const key = roomKeyInput.value.trim()
  if (!isValidRoomCode(code)) {
    error.value = 'El código de sala debe tener seis dígitos.'
    return
  }
  if (!isValidRoomKey(key)) {
    error.value = 'La clave de sala no es válida. Abre el enlace de invitación.'
    return
  }
  error.value = ''
  // The key is kept so the player can share the invite link again.
  roomKey.value = key
  client.joinRoom(playerName.value.trim() || undefined, code, key)
}

/** leaveRoom returns to the lobby. */
function leaveRoom(): void {
  client.leaveRoom()
  snapshot.value = null
  playerId.value = ''
  room.value = ''
  selectedKind.value = null
}

// opponentAway is true when the other player lost the connection.
const opponentAway = computed(() => {
  const players = snapshot.value?.players ?? []
  return players.some((player) => player.disconnected)
})

/** inviteLink is the URL the opponent can use to join the room. It carries
 * the code and the secret key, so sharing the link is enough. */
const inviteLink = computed(() =>
  room.value && roomKey.value
    ? buildInviteLink(window.location.origin, room.value, roomKey.value)
    : '',
)

/** copyInvite copies the invite link to the clipboard. */
async function copyInvite(): Promise<void> {
  try {
    await navigator.clipboard.writeText(inviteLink.value)
  } catch {
    error.value = 'No se pudo copiar. Copia el enlace a mano.'
  }
}

function placeShip(x: number, y: number): void {
  const kind = selectedKind.value
  if (!kind) {
    error.value = 'Elige un barco de la flota antes de colocarlo.'
    return
  }
  // A ship that is already on the board must not be placed again: the server
  // would answer "el portaaviones ya fue colocado".
  if (placedKinds.value.has(kind)) {
    error.value = 'Ese barco ya está colocado. Elige otro.'
    return
  }
  client.place(kind, x, y, orientation.value)
}

function onOwnClick(x: number, y: number): void {
  if (phase.value === 'setup') {
    placeShip(x, y)
    return
  }
  if (phase.value === 'playing' && isMyTurn.value) {
    client.shoot(x, y)
  }
}

function onEnemyClick(x: number, y: number): void {
  if (phase.value === 'playing' && isMyTurn.value) {
    client.shoot(x, y)
  }
}

function onKeydown(event: KeyboardEvent): void {
  if (!connected.value) return
  const key = event.key.toLowerCase()

  if (key >= 'a' && key <= 'j') {
    keyboardX.value = COLUMN_LABELS.indexOf(key.toUpperCase())
  } else if (key >= '1' && key <= '9') {
    keyboardY.value = Number(key) - 1
  } else if (key === '0') {
    keyboardY.value = 9
  } else {
    return
  }

  event.preventDefault()

  const x = keyboardX.value
  const y = keyboardY.value
  if (x === null || y === null) return

  if (phase.value === 'setup') {
    placeShip(x, y)
  } else if (phase.value === 'playing' && isMyTurn.value) {
    client.shoot(x, y)
  }
}

function selectShip(kind: ShipKind): void {
  selectedKind.value = kind
}

function removeShip(kind: ShipKind): void {
  client.removeShip(kind)
  if (selectedKind.value === kind) selectedKind.value = null
}

function ready(): void {
  client.ready()
}

function restart(): void {
  client.start()
}

onMounted(() => {
  // The socket is opened once: creating or joining a room only sends a
  // message over the existing connection.
  client.connect()
  client.onWelcome((id, joinedRoom, key) => {
    playerId.value = id
    reconnecting.value = false
    // The room field carries the six-digit code of the room.
    room.value = joinedRoom
    // The server only sends the key to the player that created the room. The
    // player that joined already knows it, so the existing value is kept.
    if (key) {
      roomKey.value = key
    }
  })
  client.onState((state) => {
    snapshot.value = state
    error.value = ''
  })
  client.onError((message) => {
    error.value = message
  })
  client.onClose(() => {
    // The client reconnects on its own: the match is kept for a while.
    reconnecting.value = true
  })
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  client.disconnect()
})
</script>

<template>
  <main class="app">
    <header class="app__header">
      <h1 class="app__title">MIDWAY</h1>
      <p class="app__subtitle">Batalla Naval · 2 jugadores</p>
    </header>

    <p v-if="reconnecting" class="app__notice" data-reconnecting>
      Conexión perdida. Reconectando…
    </p>

    <p v-if="opponentAway" class="app__notice" data-opponent-away>
      Tu rival se ha desconectado. La partida se conserva un rato.
    </p>

    <p v-if="error" class="app__error" data-error>{{ error }}</p>

    <section v-if="!connected" class="app__join" data-join>
      <h2 class="app__join-title">Entra a la partida</h2>

      <input
        v-model="playerName"
        class="app__input"
        type="text"
        placeholder="Tu nombre"
        maxlength="16"
        data-name
      />

      <form class="app__join-form" @submit.prevent="createRoom">
        <button class="app__button" type="submit" data-create>Crear sala</button>
      </form>

      <p class="app__join-separator">o</p>

      <form class="app__join-form app__join-form--join" @submit.prevent="joinRoom">
        <input
          v-model="roomCodeInput"
          class="app__input app__input--code"
          type="text"
          inputmode="numeric"
          maxlength="6"
          placeholder="Código"
          data-room-code
        />
        <input
          v-model="roomKeyInput"
          class="app__input app__input--key"
          type="text"
          autocomplete="off"
          maxlength="32"
          placeholder="Clave"
          data-room-key
        />
        <button class="app__button app__button--ghost" type="submit" data-join-room>
          Unirse
        </button>
      </form>

      <p class="app__hint">
        Abre el enlace de invitación de tu rival, o escribe su código y su clave.
      </p>
    </section>

    <template v-else-if="phase === 'setup'">
      <div v-if="room" class="app__room" data-room>
        <span class="app__room-label">Sala</span>
        <strong class="app__room-code" data-room-code-display>{{ room }}</strong>
        <code class="app__room-link" data-invite-link>{{ inviteLink }}</code>
        <span v-if="!roomKey" class="app__room-hint">
          Comparte el enlace para que tu rival entre.
        </span>
        <button class="app__button app__button--ghost" type="button" @click="copyInvite">
          Copiar enlace
        </button>
        <button class="app__button app__button--ghost" type="button" data-leave @click="leaveRoom">
          Salir
        </button>
      </div>

      <div class="app__layout">
        <div class="app__boards">
          <div class="app__board">
            <h2 class="app__board-title">Tu mar</h2>
            <BoardGrid
              :board="ownBoard"
              :show-ships="true"
              :interactive="true"
              :player-color="myColor"
              @shoot="onOwnClick"
            />
          </div>
          <div class="app__board">
            <h2 class="app__board-title">Mar enemigo</h2>
            <BoardGrid
              :board="enemyBoard"
              :show-ships="false"
              :interactive="false"
              :player-color="enemyColor"
            />
          </div>
        </div>

        <div class="app__side" data-fleet>
          <FleetPanel
            :fleet="myFleet"
            :placed="placedShips"
            :orientation="orientation"
            :can-ready="canReady"
            :selected="selectedKind"
            @update:orientation="orientation = $event"
            @select="selectShip"
            @remove="removeShip"
            @ready="ready"
          />
        </div>
      </div>
    </template>

    <template v-else-if="phase === 'playing'">
      <div v-if="room" class="app__room" data-room>
        <span class="app__room-label">Sala</span>
        <strong class="app__room-code" data-room-code-display>{{ room }}</strong>
        <code class="app__room-link" data-invite-link>{{ inviteLink }}</code>
        <span v-if="!roomKey" class="app__room-hint">
          Comparte el enlace para que tu rival entre.
        </span>
        <button class="app__button app__button--ghost" type="button" @click="copyInvite">
          Copiar enlace
        </button>
        <button class="app__button app__button--ghost" type="button" data-leave @click="leaveRoom">
          Salir
        </button>
      </div>

      <HudPanel
        :phase="phase"
        :players="players"
        :current-turn="snapshot?.currentTurn ?? ''"
        :player-id="playerId"
        :winner="snapshot?.winner ?? ''"
        :own-sunk="ownBoard.sunkShips.length"
        :enemy-sunk="enemyBoard.sunkShips.length"
        @restart="restart"
      />

      <div class="app__boards">
        <div class="app__board">
          <h2 class="app__board-title">Tu mar</h2>
          <BoardGrid
            :board="ownBoard"
            :show-ships="true"
            :interactive="false"
            :player-color="myColor"
          />
        </div>
        <div class="app__board">
          <h2 class="app__board-title">Mar enemigo</h2>
          <BoardGrid
            :board="enemyBoard"
            :show-ships="false"
            :interactive="isMyTurn"
            :player-color="enemyColor"
            @shoot="onEnemyClick"
          />
        </div>
      </div>

      <p class="app__hint">
        {{ isMyTurn ? 'Tu turno: dispara con el ratón o con las teclas A-J y 1-10.' : 'Esperando al rival…' }}
      </p>
    </template>

    <section v-else class="app__gameover" data-gameover>
      <h2 class="app__gameover-title">
        {{ winnerName === playerName || snapshot?.winner === playerId ? '¡Victoria!' : 'Derrota' }}
      </h2>
      <p class="app__gameover-text">Ganó {{ winnerName || 'el rival' }}</p>

      <div class="app__boards app__boards--final">
        <div class="app__board">
          <h2 class="app__board-title">Tu mar</h2>
          <BoardGrid
            :board="ownBoard"
            :show-ships="true"
            :interactive="false"
            :player-color="myColor"
          />
        </div>
        <div class="app__board">
          <h2 class="app__board-title">Mar enemigo</h2>
          <BoardGrid
            :board="enemyBoard"
            :show-ships="false"
            :interactive="false"
            :player-color="enemyColor"
          />
        </div>
      </div>

      <button class="app__button" type="button" data-restart @click="restart">
        Reiniciar
      </button>
    </section>
  </main>
</template>

<style scoped>
.app {
  min-height: 100vh;
  padding: 1.5rem;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 1rem;
}

.app__header {
  text-align: center;
  flex: 0 0 auto;
}

.app__title {
  margin: 0;
  font-size: 2.2rem;
  letter-spacing: 0.3em;
  color: #ffd447;
  text-shadow: 2px 2px 0 #0c0e14;
}

.app__subtitle {
  margin: 0.25rem 0 0;
  color: #8b93a7;
}

.app__notice {
  margin: 0 auto;
  padding: 0.5rem 0.75rem;
  border: 2px solid #0c0e14;
  background: #2a2f3d;
  color: #ffd447;
  font-size: 0.85rem;
  text-align: center;
}

.app__error {
  margin: 0 auto;
  padding: 0.5rem 0.75rem;
  border: 2px solid #0c0e14;
  background: #e94f6d;
  color: #12141b;
  font-weight: 700;
  text-align: center;
}

.app__join {
  align-self: center;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1.5rem;
  border: 2px solid #0c0e14;
  background: #202533;
  text-align: center;
}

.app__join-title {
  margin: 0;
  font-size: 1.1rem;
  color: #ffd447;
}

.app__join-form {
  display: flex;
  gap: 0.5rem;
  align-items: stretch;
  justify-content: center;
}

/* The room code and its key sit together, centred. */
.app__join-form--join {
  justify-content: center;
}

.app__join-separator {
  margin: 0;
  color: var(--muted-foreground, #8b93a7);
  font-size: 0.8rem;
  text-align: center;
}

.app__input--code {
  width: 11ch;
  text-align: center;
  letter-spacing: 0.25em;
  font-family: inherit;
}

.app__button--ghost {
  background: transparent;
  color: var(--foreground, #e6e9f2);
}

.app__room {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
  justify-content: center;
  padding: 0.5rem 0.75rem;
  border: 2px solid #0c0e14;
  background: var(--card, #202533);
}

.app__room-label {
  color: var(--muted-foreground, #8b93a7);
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.15em;
}

.app__room-code {
  color: var(--accent, #ffd447);
  font-size: 1.4rem;
  letter-spacing: 0.3em;
}

.app__room-link {
  color: var(--muted-foreground, #8b93a7);
  font-size: 0.75rem;
}

.app__room-hint {
  color: var(--muted-foreground, #8b93a7);
  font-size: 0.75rem;
}

.app__input--key {
  width: 22ch;
  font-size: 0.8rem;
}

.app__input {
  padding: 0.5rem 0.75rem;
  border: 2px solid #0c0e14;
  background: #12141b;
  color: inherit;
  font: inherit;
}

.app__button {
  padding: 0.5rem 1rem;
  border: 2px solid #0c0e14;
  background: #ffd447;
  color: #12141b;
  font: inherit;
  font-weight: 700;
  cursor: pointer;
}

.app__button:hover {
  background: #9ad9ff;
}

.app__layout {
  display: flex;
  gap: 1.25rem;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: center;
  flex: 1 1 auto;
}

.app__boards {
  display: flex;
  gap: 1.25rem;
  flex-wrap: wrap;
  justify-content: center;
  align-items: flex-start;
  flex: 1 1 auto;
}

.app__board {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  /* The board takes the space it can so the canvas scales up: the cell size
     is a whole number of pixels, so the board grows in jumps. */
  flex: 1 1 240px;
  /* Room for the board at 2x, gutter included. */
  max-width: 560px;
}

.app__board-title {
  margin: 0;
  font-size: 0.85rem;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: #8b93a7;
}

.app__side {
  display: flex;
}

.app__hint {
  margin: 0;
  text-align: center;
  color: #8b93a7;
  font-size: 0.85rem;
}

.app__gameover {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
  align-self: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
  padding: 2rem;
  border: 2px solid #0c0e14;
  background: #202533;
}

.app__gameover-title {
  margin: 0;
  font-size: 1.8rem;
  color: #ffd447;
}

.app__gameover-text {
  margin: 0;
  color: #e6e9f2;
}
</style>
