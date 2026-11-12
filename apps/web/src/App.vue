<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import BoardGrid from './components/BoardGrid.vue'
import FleetPanel from './components/FleetPanel.vue'
import HudPanel from './components/HudPanel.vue'
import { GameClient } from './game/client'
import { COLUMN_LABELS } from './game/board'
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
const room = ref('')
const playerName = ref('')
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

function connect(): void {
  error.value = ''
  client.connect(playerName.value.trim() || undefined)
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
  client.onWelcome((id, joinedRoom) => {
    playerId.value = id
    room.value = joinedRoom
  })
  client.onState((state) => {
    snapshot.value = state
    error.value = ''
  })
  client.onError((message) => {
    error.value = message
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

    <p v-if="error" class="app__error" data-error>{{ error }}</p>

    <section v-if="!connected" class="app__join" data-join>
      <h2 class="app__join-title">Entra a la partida</h2>
      <form class="app__join-form" @submit.prevent="connect">
        <input
          v-model="playerName"
          class="app__input"
          type="text"
          placeholder="Tu nombre"
          maxlength="16"
          data-name
        />
        <button class="app__button" type="submit" data-enter>Entrar</button>
      </form>
      <p class="app__hint">Necesitas un rival conectado para empezar.</p>
    </section>

    <template v-else-if="phase === 'setup'">
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
  gap: 1rem;
}

.app__header {
  text-align: center;
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
  gap: 1rem;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: center;
}

.app__boards {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
  justify-content: center;
}

.app__board {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
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
