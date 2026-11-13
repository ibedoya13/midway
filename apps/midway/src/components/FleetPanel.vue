<script setup lang="ts">
import { computed } from 'vue'

import type { Orientation, ShipKind, ShipSpec, ShipView } from '../game/types'

const props = defineProps<{
  fleet: ShipSpec[]
  placed: ShipView[]
  orientation: Orientation
  canReady: boolean
  selected?: ShipKind | null
}>()

const emit = defineEmits<{
  (event: 'update:orientation', value: Orientation): void
  (event: 'remove', kind: ShipKind): void
  (event: 'ready'): void
  (event: 'select', kind: ShipKind): void
}>()

const placedKinds = computed(() => new Set(props.placed.map((ship) => ship.kind)))

const pending = computed(() => props.fleet.filter((spec) => !placedKinds.value.has(spec.kind)))
</script>

<template>
  <aside class="fleet">
    <h2 class="fleet__title">Flota</h2>

    <ul class="fleet__list" data-pending>
      <li
        v-for="ship in pending"
        :key="ship.kind"
        class="fleet__item"
        :class="{ 'fleet__item--selected': selected === ship.kind }"
        :data-ship="ship.kind"
        @click="emit('select', ship.kind)"
      >
        <span class="fleet__name">{{ ship.name }}</span>
        <span class="fleet__size">{{ ship.size }}</span>
      </li>
    </ul>

    <div class="fleet__orientation">
      <span class="fleet__label">Orientación</span>
      <div class="fleet__buttons">
        <button
          type="button"
          class="fleet__button"
          data-orientation="horizontal"
          :class="{ 'fleet__button--active': orientation === 'horizontal' }"
          @click="emit('update:orientation', 'horizontal')"
        >
          Horizontal
        </button>
        <button
          type="button"
          class="fleet__button"
          data-orientation="vertical"
          :class="{ 'fleet__button--active': orientation === 'vertical' }"
          @click="emit('update:orientation', 'vertical')"
        >
          Vertical
        </button>
      </div>
    </div>

    <ul v-if="placed.length" class="fleet__placed">
      <li v-for="ship in placed" :key="ship.kind" class="fleet__placed-item">
        <span>{{ ship.name }}</span>
        <button
          type="button"
          class="fleet__button fleet__button--danger"
          :data-remove="ship.kind"
          @click="emit('remove', ship.kind)"
        >
          Quitar
        </button>
      </li>
    </ul>

    <button
      type="button"
      class="fleet__ready"
      data-ready
      :disabled="!canReady"
      @click="emit('ready')"
    >
      ¡Listo!
    </button>
  </aside>
</template>

<style scoped>
.fleet {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1rem;
  border: 2px solid #0c0e14;
  background: #202533;
  min-width: 220px;
}

.fleet__title {
  margin: 0;
  font-size: 1rem;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: #ffd447;
}

.fleet__list,
.fleet__placed {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.fleet__item,
.fleet__placed-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.35rem 0.5rem;
  border: 1px solid #3a4157;
  background: #12141b;
}

.fleet__item--selected {
  border-color: #ffd447;
  color: #ffd447;
  cursor: pointer;
}

.fleet__size {
  font-weight: 700;
  color: #ffd447;
}

.fleet__label {
  display: block;
  font-size: 0.75rem;
  color: #8b93a7;
  margin-bottom: 0.25rem;
}

.fleet__buttons {
  display: flex;
  gap: 0.5rem;
}

.fleet__button {
  flex: 1;
  padding: 0.35rem 0.5rem;
  border: 1px solid #3a4157;
  background: #12141b;
  color: #e6e9f2;
  font: inherit;
  cursor: pointer;
}

.fleet__button--active {
  border-color: #ffd447;
  color: #ffd447;
}

.fleet__button--danger {
  flex: 0 0 auto;
  color: #e94f6d;
}

.fleet__ready {
  margin-top: auto;
  padding: 0.6rem;
  border: 2px solid #0c0e14;
  background: #ffd447;
  color: #12141b;
  font: inherit;
  font-weight: 700;
  cursor: pointer;
}

.fleet__ready:disabled {
  background: #3a4157;
  color: #8b93a7;
  cursor: not-allowed;
}
</style>
