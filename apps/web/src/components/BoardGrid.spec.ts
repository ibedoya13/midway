import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import BoardGrid from './BoardGrid.vue'
import { TILE_SIZE } from '../game/renderer'
import type { BoardView } from '../game/types'

const board: BoardView = {
  width: 10,
  height: 10,
  ships: [],
  sunkShips: [],
  shots: [],
}

function stubCanvas(): { ctx: Record<string, unknown>; fillRect: ReturnType<typeof vi.fn> } {
  const fillRect = vi.fn()
  const ctx = { imageSmoothingEnabled: true, fillStyle: '#000', fillRect }
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
    ctx as unknown as CanvasRenderingContext2D,
  )
  return { ctx, fillRect }
}

function clickCell(wrapper: ReturnType<typeof mount>, x: number, y: number): void {
  const canvas = wrapper.find('canvas')
  const event = new MouseEvent('click', { bubbles: true })
  Object.defineProperty(event, 'offsetX', { value: x * TILE_SIZE + 3 })
  Object.defineProperty(event, 'offsetY', { value: y * TILE_SIZE + 4 })
  canvas.element.dispatchEvent(event)
}

describe('BoardGrid', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('renders a canvas with the logical board size', () => {
    stubCanvas()
    const wrapper = mount(BoardGrid, {
      props: { board, showShips: true, interactive: false },
    })
    const canvas = wrapper.find('canvas')
    expect(canvas.attributes('width')).toBe(String(10 * TILE_SIZE))
    expect(canvas.attributes('height')).toBe(String(10 * TILE_SIZE))
  })

  it('paints the board when mounted', () => {
    const { fillRect } = stubCanvas()
    mount(BoardGrid, { props: { board, showShips: true, interactive: false } })
    expect(fillRect).toHaveBeenCalled()
  })

  it('emits shoot with the clicked cell', () => {
    stubCanvas()
    const wrapper = mount(BoardGrid, {
      props: { board, showShips: false, interactive: true },
    })
    clickCell(wrapper, 3, 4)
    expect(wrapper.emitted('shoot')).toEqual([[3, 4]])
  })

  it('does not emit shoot when the board is not interactive', () => {
    stubCanvas()
    const wrapper = mount(BoardGrid, {
      props: { board, showShips: false, interactive: false },
    })
    clickCell(wrapper, 3, 4)
    expect(wrapper.emitted('shoot')).toBeUndefined()
  })

  it('scales the canvas by an integer factor to keep the pixel art crisp', () => {
    stubCanvas()
    const wrapper = mount(BoardGrid, {
      props: { board, showShips: false, interactive: false },
      attachTo: document.body,
    })
    const canvas = wrapper.find('canvas')
    const width = Number(canvas.attributes('style')?.match(/width:\s*([\d.]+)px/)?.[1] ?? 0)
    const logical = 10 * TILE_SIZE
    expect(width).toBeGreaterThan(0)
    expect(width % logical).toBe(0)
    wrapper.unmount()
  })

  it('repaints when the board changes', async () => {
    const { fillRect } = stubCanvas()
    const wrapper = mount(BoardGrid, {
      props: { board, showShips: false, interactive: false },
    })
    const before = fillRect.mock.calls.length
    await wrapper.setProps({
      board: { ...board, shots: [{ x: 0, y: 0, result: 'water' }] },
    })
    expect(fillRect.mock.calls.length).toBeGreaterThan(before)
  })
})
