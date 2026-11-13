// Checks that the production bundle points to the deployed backend.
//
// The WebSocket URL is embedded at build time, so a build made without
// VITE_WS_URL ships a client that cannot reach the server. This check fails
// loudly instead of letting that bundle reach production.
import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'

// The variable is checked BEFORE the build: a build without it would
// overwrite dist/ with a bundle that cannot reach the server.
const expected = process.env.VITE_WS_URL
if (!expected) {
  console.error(
    'build:prod: falta VITE_WS_URL. Sin ella el bundle apunta a la red local.',
  )
  process.exit(1)
}

// In the precheck the bundle does not exist yet: only the variable matters.
if (process.argv.includes('--precheck')) {
  console.log(`build:prod: VITE_WS_URL=${expected}`)
  process.exit(0)
}

const dist = new URL('../dist/assets/', import.meta.url).pathname

let files
try {
  files = readdirSync(dist).filter((name) => name.endsWith('.js'))
} catch {
  console.error('build:prod: no hay dist/assets. Ejecuta el build primero.')
  process.exit(1)
}

if (files.length === 0) {
  console.error('build:prod: dist/assets no tiene bundles JS.')
  process.exit(1)
}

for (const file of files) {
  const content = readFileSync(join(dist, file), 'utf8')
  if (!content.includes(expected)) {
    console.error(
      `build:prod: ${file} no contiene ${expected}.`,
      'El bundle se construyo sin VITE_WS_URL o con otra URL.',
    )
    process.exit(1)
  }
}

console.log(`build:prod: ok, ${files.length} bundle(s) con ${expected}`)
