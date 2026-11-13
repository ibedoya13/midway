import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import FleetPanel from './FleetPanel.vue'
import type { ShipSpec } from '../game/types'

const fleet: ShipSpec[] = [
  { kind: 'carrier', name: 'Portaaviones', size: 5 },
  { kind: 'battleship', name: 'Acorazado', size: 4 },
  { kind: 'cruiser', name: 'Crucero', size: 3 },
  { kind: 'submarine', name: 'Submarino', size: 3 },
  { kind: 'destroyer', name: 'Destructor', size: 2 },
]

function mountPanel(props: Partial<InstanceType<typeof FleetPanel>['$props']> = {}) {
  return mount(FleetPanel, {
    props: {
      fleet,
      placed: [],
      orientation: 'horizontal',
      canReady: false,
      ...props,
    },
  })
}

describe('FleetPanel', () => {
  it('lists every ship of the fleet with its size', () => {
    const wrapper = mountPanel()
    const text = wrapper.text()
    expect(text).toContain('Portaaviones')
    expect(text).toContain('Acorazado')
    expect(text).toContain('Crucero')
    expect(text).toContain('Submarino')
    expect(text).toContain('Destructor')
    expect(text).toContain('5')
    expect(text).toContain('2')
  })

  it('shows only the pending ships', () => {
    const wrapper = mountPanel({
      placed: [{ kind: 'carrier', name: 'Portaaviones', size: 5, x: 0, y: 0, orientation: 'horizontal' }],
    })
    const items = wrapper.findAll('[data-ship]')
    expect(items).toHaveLength(4)
    expect(wrapper.find('[data-pending]').text()).not.toContain('Portaaviones')
  })

  it('emits update:orientation when the orientation changes', async () => {
    const wrapper = mountPanel()
    await wrapper.find('[data-orientation="vertical"]').trigger('click')
    expect(wrapper.emitted('update:orientation')).toEqual([['vertical']])
  })

  it('emits remove for a placed ship', async () => {
    const wrapper = mountPanel({
      placed: [{ kind: 'cruiser', name: 'Crucero', size: 3, x: 1, y: 1, orientation: 'vertical' }],
    })
    await wrapper.find('[data-remove="cruiser"]').trigger('click')
    expect(wrapper.emitted('remove')).toEqual([['cruiser']])
  })

  it('disables the ready button until the whole fleet is placed', () => {
    const wrapper = mountPanel()
    expect(wrapper.find('[data-ready]').attributes('disabled')).toBeDefined()
  })

  it('enables the ready button when the fleet is complete', () => {
    const wrapper = mountPanel({
      canReady: true,
      placed: fleet.map((spec, index) => ({
        ...spec,
        x: index,
        y: 0,
        orientation: 'horizontal' as const,
      })),
    })
    expect(wrapper.find('[data-ready]').attributes('disabled')).toBeUndefined()
  })

  it('emits ready when the button is pressed', async () => {
    const wrapper = mountPanel({ canReady: true })
    await wrapper.find('[data-ready]').trigger('click')
    expect(wrapper.emitted('ready')).toHaveLength(1)
  })
})
