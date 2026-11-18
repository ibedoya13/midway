import { describe, expect, it, vi } from 'vitest'

import { GameAudio, SOUNDS, type AudioPlayer } from './audio'

/** A player that records what was asked of it. */
type TrackedPlayer = AudioPlayer & { played: number; paused: number }

/** A player that records what was asked of it. */
function makePlayer(): TrackedPlayer {
  const player = {
    played: 0,
    paused: 0,
    currentTime: 0,
    loop: false,
    volume: 1,
    preload: 'auto',
    src: '',
    play: vi.fn(() => {
      player.played += 1
      return Promise.resolve()
    }),
    pause: vi.fn(() => {
      player.paused += 1
    }),
  }
  return player
}

describe('GameAudio', () => {
  it('plays the battle theme in a loop when the match starts', () => {
    const players: Record<string, TrackedPlayer> = {}
    const audio = new GameAudio((src) => {
      const player = makePlayer()
      player.src = src
      players[src] = player
      return player
    })

    audio.startTheme()

    expect(players[SOUNDS.theme]).toBeDefined()
    expect(players[SOUNDS.theme].loop).toBe(true)
    expect(players[SOUNDS.theme].played).toBe(1)
  })

  it('does not start the theme twice', () => {
    const players: Record<string, TrackedPlayer> = {}
    const audio = new GameAudio((src) => {
      const player = makePlayer()
      player.src = src
      players[src] = player
      return player
    })

    audio.startTheme()
    audio.startTheme()

    expect(players[SOUNDS.theme].played).toBe(1)
  })

  it('pauses the theme when the match is over', () => {
    const players: Record<string, TrackedPlayer> = {}
    const audio = new GameAudio((src) => {
      const player = makePlayer()
      player.src = src
      players[src] = player
      return player
    })

    audio.startTheme()
    audio.stopTheme()

    expect(players[SOUNDS.theme].paused).toBe(1)
  })

  it('plays the hit sound and the miss sound', () => {
    const players: Record<string, TrackedPlayer> = {}
    const audio = new GameAudio((src) => {
      const player = makePlayer()
      player.src = src
      players[src] = player
      return player
    })

    audio.playHit()
    audio.playMiss()

    expect(players[SOUNDS.hit].played).toBe(1)
    expect(players[SOUNDS.miss].played).toBe(1)
  })

  it('survives a browser that refuses to play', async () => {
    const audio = new GameAudio(() => {
      const player = makePlayer()
      player.play = vi.fn(() => Promise.reject(new Error('NotAllowedError')))
      return player
    })

    expect(() => {
      audio.startTheme()
      audio.playHit()
      audio.playMiss()
    }).not.toThrow()
    await Promise.resolve()
  })

  it('retries the theme after the player unlocks the audio', async () => {
    const players: Record<string, TrackedPlayer> = {}
    let blocked = true
    const audio = new GameAudio((src) => {
      const player = makePlayer()
      player.src = src
      players[src] = player
      if (blocked) {
        player.play = vi.fn(() => {
          player.played += 1
          return Promise.reject(new Error('NotAllowedError'))
        })
      }
      return player
    })

    audio.startTheme()
    await Promise.resolve()
    await Promise.resolve()
    blocked = false
    audio.unlock()

    expect(players[SOUNDS.theme].played).toBe(2)
  })

  it('does not restart a theme that is already playing', async () => {
    const players: Record<string, TrackedPlayer> = {}
    const audio = new GameAudio((src) => {
      const player = makePlayer()
      player.src = src
      players[src] = player
      return player
    })

    audio.startTheme()
    await Promise.resolve()
    await Promise.resolve()
    audio.unlock()

    expect(players[SOUNDS.theme].played).toBe(1)
  })

  it('does not start the theme when the player unlocks the audio outside a match', () => {
    const players: Record<string, TrackedPlayer> = {}
    const audio = new GameAudio((src) => {
      const player = makePlayer()
      player.src = src
      players[src] = player
      return player
    })

    audio.unlock()

    expect(players[SOUNDS.theme]).toBeUndefined()
  })

  it('does not download the theme until it is needed', () => {
    const factory = vi.fn((src: string) => {
      const player = makePlayer()
      player.src = src
      return player
    })
    const audio = new GameAudio(factory)

    expect(factory).not.toHaveBeenCalled()

    audio.warmUp()

    expect(factory).toHaveBeenCalledWith(SOUNDS.theme)
  })
})
