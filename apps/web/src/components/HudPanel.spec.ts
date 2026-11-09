import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import HudPanel from './HudPanel.vue'
import type { PlayerInfo } from '../game/types'

const players: PlayerInfo[] = [
  { id: 'p1', name: 'Ana', color: '#e94f6d', ready: true },
  { id: 'p2', name: 'Beto', color: '#4f8ef7', ready: false },
]

describe('HudPanel', () => {
  it('shows the current phase', () => {
    const wrapper = mount(HudPanel, {
      props: { phase: 'setup', players, currentTurn: '', playerId: 'p1', winner: '' },
    })
    expect(wrapper.text()).toContain('Colocación')
  })

  it('shows both players with their names', () => {
    const wrapper = mount(HudPanel, {
      props: { phase: 'playing', players, currentTurn: 'p1', playerId: 'p1', winner: '' },
    })
    expect(wrapper.text()).toContain('Ana')
    expect(wrapper.text()).toContain('Beto')
  })

  it('marks whose turn it is', () => {
    const wrapper = mount(HudPanel, {
      props: { phase: 'playing', players, currentTurn: 'p2', playerId: 'p1', winner: '' },
    })
    const turn = wrapper.find('[data-turn]')
    expect(turn.exists()).toBe(true)
    expect(turn.text()).toContain('Beto')
  })

  it('shows the score of sunk ships', () => {
    const wrapper = mount(HudPanel, {
      props: {
        phase: 'playing',
        players,
        currentTurn: 'p1',
        playerId: 'p1',
        winner: '',
        ownSunk: 1,
        enemySunk: 2,
      },
    })
    expect(wrapper.text()).toContain('1')
    expect(wrapper.text()).toContain('2')
  })

  it('shows the winner when the game is over', () => {
    const wrapper = mount(HudPanel, {
      props: { phase: 'gameover', players, currentTurn: '', playerId: 'p1', winner: 'p1' },
    })
    expect(wrapper.text()).toContain('Ana')
    expect(wrapper.text().toLowerCase()).toContain('gan')
  })

  it('emits restart when the restart button is pressed', async () => {
    const wrapper = mount(HudPanel, {
      props: { phase: 'playing', players, currentTurn: 'p1', playerId: 'p1', winner: '' },
    })
    await wrapper.find('[data-restart]').trigger('click')
    expect(wrapper.emitted('restart')).toHaveLength(1)
  })
})
