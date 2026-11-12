import { describe, expect, it } from 'vitest'

import { DEFAULT_BACKEND_PORT, resolveWebSocketUrl } from './url'

describe('resolveWebSocketUrl', () => {
  it('uses the configured url when it is set', () => {
    expect(resolveWebSocketUrl('192.168.0.2:5173', 'ws://example.com/ws')).toBe(
      'ws://example.com/ws',
    )
  })

  it('keeps localhost when the page is opened locally', () => {
    expect(resolveWebSocketUrl('localhost:5173')).toBe(
      `ws://localhost:${DEFAULT_BACKEND_PORT}/ws`,
    )
  })

  it('uses the lan host of the page so other devices reach the server', () => {
    expect(resolveWebSocketUrl('192.168.0.2:5173')).toBe('ws://192.168.0.2:8080/ws')
  })

  it('ignores the port of the page', () => {
    expect(resolveWebSocketUrl('10.0.0.5:4173')).toBe('ws://10.0.0.5:8080/ws')
  })

  it('honours a custom backend port', () => {
    expect(resolveWebSocketUrl('192.168.0.2:5173', undefined, 9000)).toBe(
      'ws://192.168.0.2:9000/ws',
    )
  })

  it('falls back to localhost without a host', () => {
    expect(resolveWebSocketUrl('')).toBe(`ws://localhost:${DEFAULT_BACKEND_PORT}/ws`)
  })
})
