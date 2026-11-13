import { describe, expect, it } from 'vitest'

import {
  buildInviteLink,
  isValidRoomCode,
  isValidRoomKey,
  roomCodeFromUrl,
  roomKeyFromUrl,
  roomParamsFromUrl,
} from './room'

describe('isValidRoomCode', () => {
  it('accepts six digits', () => {
    expect(isValidRoomCode('123456')).toBe(true)
  })

  it('rejects anything else', () => {
    for (const bad of ['', '12345', '1234567', '12345a', 'abcdef', ' 123456 ']) {
      expect(isValidRoomCode(bad)).toBe(false)
    }
  })
})

describe('isValidRoomKey', () => {
  it('accepts a key of 32 hex characters', () => {
    expect(isValidRoomKey('a'.repeat(32))).toBe(true)
    expect(isValidRoomKey('0123456789abcdef'.repeat(2))).toBe(true)
  })

  it('rejects anything else', () => {
    expect(isValidRoomKey('')).toBe(false)
    expect(isValidRoomKey('a'.repeat(31))).toBe(false)
    expect(isValidRoomKey('a'.repeat(33))).toBe(false)
    expect(isValidRoomKey('z'.repeat(32))).toBe(false)
  })
})

describe('roomParamsFromUrl', () => {
  it('reads both the code and the key', () => {
    const key = 'b'.repeat(32)
    expect(roomParamsFromUrl(`https://midway.amarrete.com/?room=123456&key=${key}`)).toEqual({
      code: '123456',
      key,
    })
  })

  it('drops a malformed key but keeps the code', () => {
    expect(roomParamsFromUrl('https://midway.amarrete.com/?room=123456&key=short')).toEqual({
      code: '123456',
      key: '',
    })
  })
})

describe('roomKeyFromUrl', () => {
  it('reads the key of the query string', () => {
    const key = 'c'.repeat(32)
    expect(roomKeyFromUrl(`https://midway.amarrete.com/?room=123456&key=${key}`)).toBe(key)
  })

  it('returns an empty string when there is no key', () => {
    expect(roomKeyFromUrl('https://midway.amarrete.com/?room=123456')).toBe('')
  })
})

describe('buildInviteLink', () => {
  it('carries the code and the key', () => {
    const key = 'd'.repeat(32)
    expect(buildInviteLink('https://midway.amarrete.com', '123456', key)).toBe(
      `https://midway.amarrete.com/?room=123456&key=${key}`,
    )
  })
})

describe('roomCodeFromUrl', () => {
  it('reads the room parameter of the query string', () => {
    expect(roomCodeFromUrl('https://midway.amarrete.com/?room=123456')).toBe('123456')
  })

  it('returns an empty string when there is no room', () => {
    expect(roomCodeFromUrl('https://midway.amarrete.com/')).toBe('')
  })

  it('ignores a malformed room parameter', () => {
    expect(roomCodeFromUrl('https://midway.amarrete.com/?room=12')).toBe('')
  })
})
