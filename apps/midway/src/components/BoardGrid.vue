<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import { canvasSize, pixelToTile, renderBoard } from '../game/renderer'
import type { BoardView } from '../game/types'

const props = withDefaults(
  defineProps<{
    board: BoardView
    showShips: boolean
    interactive: boolean
    highlight?: { x: number; y: number } | null
    playerColor?: string
  }>(),
  {
    highlight: null,
    playerColor: '#e94f6d',
  },
)

const emit = defineEmits<{
  (event: 'shoot', x: number, y: number): void
}>()

const canvasRef = ref<HTMLCanvasElement | null>(null)
const wrapperRef = ref<HTMLElement | null>(null)

// The canvas holds the play area plus the gutter with the coordinates.
const logicalWidth = computed(() => canvasSize(props.board.width, props.board.height).width)
const logicalHeight = computed(() => canvasSize(props.board.width, props.board.height).height)

/**
 * The board is scaled by a whole number of pixels. A fractional scale would
 * blur the 1px grid lines and the 2px glyphs, breaking the pixel art.
 */
const scale = ref(1)

function updateScale(): void {
  const wrapper = wrapperRef.value
  if (!wrapper) return
  const available = wrapper.clientWidth
  if (available <= 0) return
  const factor = Math.max(1, Math.floor(available / logicalWidth.value))
  scale.value = factor
}

const displayWidth = computed(() => logicalWidth.value * scale.value)
const displayHeight = computed(() => logicalHeight.value * scale.value)

function paint(): void {
  const canvas = canvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  renderBoard(ctx, props.board, {
    showShips: props.showShips,
    highlight: props.highlight,
    color: props.playerColor,
  })
}

function onClick(event: MouseEvent): void {
  if (!props.interactive) return
  const canvas = canvasRef.value
  if (!canvas) return
  const rect = canvas.getBoundingClientRect()
  const scaleX = rect.width > 0 ? canvas.width / rect.width : 1
  const scaleY = rect.height > 0 ? canvas.height / rect.height : 1
  const tile = pixelToTile(
    event.offsetX * scaleX,
    event.offsetY * scaleY,
    props.board.width,
    props.board.height,
  )
  if (!tile) return
  emit('shoot', tile.x, tile.y)
}

let observer: ResizeObserver | null = null

onMounted(() => {
  updateScale()
  paint()
  if (wrapperRef.value && typeof ResizeObserver !== 'undefined') {
    observer = new ResizeObserver(() => updateScale())
    observer.observe(wrapperRef.value)
  }
})

onBeforeUnmount(() => {
  observer?.disconnect()
  observer = null
})

watch(
  () => [props.board, props.showShips, props.highlight, props.playerColor],
  () => paint(),
  { deep: true },
)

watch([displayWidth, displayHeight], () => paint())
</script>

<template>
  <div ref="wrapperRef" class="board-grid__wrapper">
    <canvas
      ref="canvasRef"
      class="board-grid"
      :width="canvasSize(board.width, board.height).width"
      :height="canvasSize(board.width, board.height).height"
      :style="{ width: `${displayWidth}px`, height: `${displayHeight}px` }"
      :class="{ 'board-grid--interactive': interactive }"
      @click="onClick"
    />
  </div>
</template>

<style scoped>
.board-grid__wrapper {
  width: 100%;
  /* Wide enough for the board to scale to 2x with its gutter. */
  max-width: 520px;
  display: flex;
  justify-content: center;
}

.board-grid {
  display: block;
  image-rendering: pixelated;
  image-rendering: crisp-edges;
  border: 2px solid #0c0e14;
  background: #12304a;
  cursor: default;
}

.board-grid--interactive {
  cursor: crosshair;
}
</style>
