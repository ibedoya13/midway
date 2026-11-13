/**
 * Helpers for the room identifiers.
 *
 * A room carries two identifiers:
 *
 *   - the code, six digits, public and readable. It names the room so players
 *     can talk about it ("sala 097344").
 *   - the key, a long random secret. It is the credential that admits a
 *     player to the room: the code alone is not enough.
 *
 * The invite link carries both, so sharing the link is enough to let the
 * opponent in: /?room=097344&key=1a2b3c...
 */

/** ROOM_CODE_LENGTH is the number of digits of a room code. */
export const ROOM_CODE_LENGTH = 6

/** ROOM_KEY_LENGTH is the number of hex characters of a room key. */
export const ROOM_KEY_LENGTH = 32

/**
 * isValidRoomCode reports whether a code has the expected format.
 */
export function isValidRoomCode(code: string): boolean {
  if (code.length !== ROOM_CODE_LENGTH) return false
  for (const char of code) {
    if (char < '0' || char > '9') return false
  }
  return true
}

/**
 * isValidRoomKey reports whether a key has the expected format.
 */
export function isValidRoomKey(key: string): boolean {
  if (key.length !== ROOM_KEY_LENGTH) return false
  for (const char of key) {
    if (!/[0-9a-f]/.test(char)) return false
  }
  return true
}

/**
 * roomParamsFromUrl reads the room code and the room key from a page URL.
 */
export function roomParamsFromUrl(href: string): { code: string; key: string } {
  const query = href.split('?')[1]?.split('#')[0] ?? ''
  const params = new URLSearchParams(query)
  const code = params.get('room') ?? ''
  const key = params.get('key') ?? ''
  return {
    code: isValidRoomCode(code) ? code : '',
    key: isValidRoomKey(key) ? key : '',
  }
}

/**
 * roomCodeFromUrl reads the room code from a page URL.
 */
export function roomCodeFromUrl(href: string): string {
  return roomParamsFromUrl(href).code
}

/**
 * roomKeyFromUrl reads the room key from a page URL.
 */
export function roomKeyFromUrl(href: string): string {
  return roomParamsFromUrl(href).key
}

/**
 * buildInviteLink returns the URL that invites the opponent to the room.
 */
export function buildInviteLink(origin: string, code: string, key: string): string {
  return `${origin}/?room=${code}&key=${key}`
}
