<script setup lang="ts">
import { computed } from 'vue'

import type { GamePhase, PlayerInfo } from '../game/types'

const props = withDefaults(
  defineProps<{
    phase: GamePhase
    players: PlayerInfo[]
    currentTurn: string
    playerId: string
    winner: string
    ownSunk?: number
    enemySunk?: number
  }>(),
  {
    ownSunk: 0,
    enemySunk: 0,
  },
)

const emit = defineEmits<{
  (event: 'restart'): void
}>()

const PHASE_LABELS: Record<GamePhase, string> = {
  setup: 'Colocación',
  playing: 'En juego',
  gameover: 'Fin de la partida',
}

const phaseLabel = computed(() => PHASE_LABELS[props.phase] ?? props.phase)

const currentPlayer = computed(
  () => props.players.find((player) => player.id === props.currentTurn) ?? null,
)

const winnerPlayer = computed(
  () => props.players.find((player) => player.id === props.winner) ?? null,
)
</script>

<template>
  <section class="hud">
    <p class="hud__phase" data-phase>{{ phaseLabel }}</p>

    <ul class="hud__players">
      <li
        v-for="player in players"
        :key="player.id"
        class="hud__player"
        :data-player="player.id"
      >
        <span class="hud__swatch" :style="{ background: player.color }" />
        <span class="hud__name">{{ player.name }}</span>
        <span v-if="player.ready" class="hud__ready">listo</span>
      </li>
    </ul>

    <p v-if="currentPlayer" class="hud__turn" data-turn>
      Turno de {{ currentPlayer.name }}
    </p>
    <p v-else class="hud__turn hud__turn--idle" data-turn>Esperando…</p>

    <p class="hud__score">
      <span class="hud__score-item">Tus hundidos: <strong>{{ ownSunk }}</strong></span>
      <span class="hud__score-item">Enemigos hundidos: <strong>{{ enemySunk }}</strong></span>
    </p>

    <p v-if="phase === 'gameover'" class="hud__winner" data-winner>
      Ganó {{ winnerPlayer ? winnerPlayer.name : 'el rival' }}
    </p>

    <button type="button" class="hud__restart" data-restart @click="emit('restart')">
      Reiniciar
    </button>
  </section>
</template>

<style scoped>
.hud {
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 1rem;
  padding: 0.6rem 0.85rem;
  border: 2px solid #0c0e14;
  background: #202533;
}

.hud__phase {
  margin: 0;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: #ffd447;
}

.hud__players {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.hud__player {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.25rem 0.45rem;
  border: 1px solid #3a4157;
  background: #12141b;
}

.hud__swatch {
  width: 12px;
  height: 12px;
  border: 2px solid #0c0e14;
  display: inline-block;
}

.hud__ready {
  margin-left: 0.2rem;
  font-size: 0.7rem;
  color: #8b93a7;
  text-transform: uppercase;
}

.hud__turn {
  margin: 0;
  padding: 0.3rem 0.55rem;
  border: 1px solid #3a4157;
  background: #12141b;
  color: #9ad9ff;
  font-size: 0.9rem;
}

.hud__turn--idle {
  color: #8b93a7;
}

.hud__score {
  margin: 0;
  display: flex;
  flex-direction: row;
  gap: 0.9rem;
  font-size: 0.85rem;
  color: #8b93a7;
}

.hud__score strong {
  color: #e6e9f2;
}

.hud__winner {
  margin: 0;
  padding: 0.5rem;
  border: 2px solid #ffd447;
  background: #12141b;
  color: #ffd447;
  font-weight: 700;
}

.hud__restart {
  margin-left: auto;
  padding: 0.3rem 0.7rem;
  border: 2px solid #3a4157;
  background: transparent;
  color: #8b93a7;
  font: inherit;
  font-size: 0.8rem;
  cursor: pointer;
}

.hud__restart:hover {
  border-color: #ffd447;
  color: #ffd447;
}
</style>
