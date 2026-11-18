/**
 * Sounds of the game.
 *
 * The browser blocks audio until the player interacts with the page, so every
 * failure to play is swallowed: a silent game is better than a broken one.
 */

/** Files of the public folder that carry the sounds. */
export const SOUNDS = {
  theme: '/midway-battle-theme.mp3',
  hit: '/hit.mp3',
  miss: '/miss.mp3',
} as const

/** The part of an audio element the game uses. */
export interface AudioPlayer {
  play(): Promise<void>
  pause(): void
  currentTime: number
  loop: boolean
  volume: number
  preload: string
  src: string
}

/** Builds the player of a file. It is injected to keep the tests simple. */
export type AudioFactory = (src: string) => AudioPlayer

function createAudio(src: string): AudioPlayer {
  const element = new Audio(src)
  element.preload = 'auto'
  return element
}

/** Volume of the battle theme, which is long and loops. */
const THEME_VOLUME = 0.35
/** Volume of the short effects. */
const EFFECT_VOLUME = 0.7

export class GameAudio {
  private readonly factory: AudioFactory
  private theme: AudioPlayer | null = null
  private hit: AudioPlayer | null = null
  private miss: AudioPlayer | null = null
  /** True while the theme is meant to be heard. */
  private themeWanted = false
  /** True while the theme is actually playing. */
  private themePlaying = false
  /** True while a play of the theme is on its way. */
  private themeStarting = false

  constructor(factory: AudioFactory = createAudio) {
    this.factory = factory
  }

  /**
   * starts downloading the theme in the background. The file is over a
   * megabyte, so it is not requested until the player enters a room.
   */
  warmUp(): void {
    if (!this.theme) {
      this.theme = this.factory(SOUNDS.theme)
      this.theme.loop = true
      this.theme.volume = THEME_VOLUME
    }
  }

  /** plays the battle theme in a loop. */
  startTheme(): void {
    this.themeWanted = true
    if (this.themePlaying || this.themeStarting) return
    this.warmUp()
    const theme = this.theme
    if (!theme) return
    this.themeStarting = true
    theme.currentTime = 0
    void this.play(theme).then((played) => {
      this.themeStarting = false
      // The match may have ended while the browser was deciding.
      if (!this.themeWanted) {
        theme.pause()
        this.themePlaying = false
        return
      }
      this.themePlaying = played
    })
  }

  /** stops the battle theme. */
  stopTheme(): void {
    this.themeWanted = false
    if (!this.themePlaying && !this.themeStarting) return
    this.theme?.pause()
    this.themePlaying = false
    this.themeStarting = false
  }

  /**
   * unlocks the audio after a click. The browser refuses to play anything
   * before the first interaction, so the theme is retried here.
   */
  unlock(): void {
    if (!this.themeWanted || this.themePlaying) return
    this.startTheme()
  }

  /** plays the sound of a shot that hit a ship. */
  playHit(): void {
    if (!this.hit) this.hit = this.factory(SOUNDS.hit)
    this.playEffect(this.hit)
  }

  /** plays the sound of a shot that fell on the water. */
  playMiss(): void {
    if (!this.miss) this.miss = this.factory(SOUNDS.miss)
    this.playEffect(this.miss)
  }

  private playEffect(player: AudioPlayer): void {
    player.volume = EFFECT_VOLUME
    player.currentTime = 0
    void this.play(player)
  }

  /** plays a file and tells whether the browser allowed it. */
  private async play(player: AudioPlayer): Promise<boolean> {
    try {
      await player.play()
      return true
    } catch {
      // The browser blocked the sound: the player has not interacted yet.
      return false
    }
  }
}
