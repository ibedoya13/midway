# Midway

Monorepo de un juego de **batalla naval** para dos jugadores con estética pixel art.

- **`apps/web`** — SPA en Vue 3 + Vite + TypeScript. Dibuja los tableros en un `<canvas>`
  con escalado nearest-neighbour (pixel art real) y habla con el backend por WebSocket.
- **`apps/server`** — Backend en Go. Lógica de juego autoritativa (colocación de flota,
  turnos, disparos, hundimientos) y servidor WebSocket (`gorilla/websocket`).

Las reglas del juego están en [`docs/game_rules.md`](docs/game_rules.md).

## Estructura

```
.
├── apps/
│   ├── server/        # Go: dominio del juego + WebSocket
│   │   ├── cmd/midway/
│   │   └── internal/
│   │       ├── game/  # tablero, flota, turnos (sin imports de transporte)
│   │       └── ws/    # protocolo y hub WebSocket
│   └── web/           # Vue 3 SPA
├── docs/game_rules.md
├── Makefile           # Atajos de comandos del monorepo
└── package.json       # Workspaces npm (apps/web)
```

## Requisitos

- Node.js >= 20
- Go >= 1.22

## Puesta en marcha

```bash
make install     # instala dependencias del frontend
make dev         # backend (:8080) + frontend (:5173) en paralelo
```

O por separado:

```bash
make dev-server  # go run ./apps/server/cmd/midway
make dev-web     # npm --workspace apps/web run dev
```

Abre <http://localhost:5173> en dos ventanas o pestañas: cada pestaña es un jugador.
Ambas deben entrar en la sala antes de poder colocar la flota.

### Jugar en la red local (Wi-Fi)

Para jugar desde el teléfono o desde otro portátil en el mismo Wi-Fi:

```bash
make dev
```

El servidor de Vite ya publica en `http://<tu-IP>:5173/`. Averigua tu IP con:

```bash
# macOS
ipconfig getifaddr en0
# Linux
hostname -I | awk '{print $1}'
```

Abre esa dirección en los dos dispositivos (por ejemplo `http://192.168.0.2:5173/`).
El frontend detecta el host con el que abriste la página y se conecta solo al
backend por `ws://<tu-IP>:8080/ws`, así que no hay nada que configurar.

Detalles a tener en cuenta:

- **El firewall de macOS** puede pedir permiso la primera vez para aceptar
  conexiones entrantes de Python/Node. Si el dispositivo no conecta, revisa
  *Ajustes del Sistema → Red → Firewall → Firewall de macOS* y permite
  `go`/`node`.
- Los dos dispositivos deben estar en la **misma red Wi-Fi** y la red no debe
  tener aislamiento de clientes activado (algunos Wi-Fi de invitados lo tienen).
- Si el router usa un rango como `192.168.0.x` pero tu IP es de otra,
  `ipconfig` te dirá la correcta; usa siempre la que Vite imprime como
  `Network:` al arrancar.
- Para fijar la URL a mano (por ejemplo, si el backend está en otra máquina),
  define `VITE_WS_URL=ws://192.168.0.2:8080/ws` antes de `make dev-web`.

## Salas

Al entrar, el jugador tiene dos opciones:

- **Crear sala.** El servidor genera un código aleatorio de seis dígitos (por
  ejemplo `097344`) y una **clave secreta** de 32 caracteres hexadecimales. El
  jugador entra como primero y ve un enlace de invitación
  (`/?room=097344&key=1a2b3c…`) para compartir.
- **Unirse.** Se escribe el código y la clave de una sala existente — o, más
  simple, se abre el enlace de invitación, que ya los trae rellenos.

Reglas de las salas:

- **El código no es una credencial.** Es público y legible (un millón de
  combinaciones), así que por sí solo **no** deja entrar: hace falta la clave
  secreta, que se genera con `crypto/rand` y se compara en tiempo constante.
  Solo el creador de la sala recibe la clave; el que se une ya la conoce. Un
  tercero que adivine el código se queda fuera.

- Pueden existir **hasta 10 salas simultáneas**. Una sala vacía se libera sola.
- **No se puede entrar en una sala cuya partida ya empezó**: el servidor
  rechaza la entrada para no romper el orden de turnos ni los tableros de los
  jugadores que ya están jugando.
- Cada sala es un juego independiente: los jugadores de una sala nunca ven el
  estado de otra.
- El botón **Salir** abandona la sala y devuelve al vestíbulo. Si se sale a
  mitad de una partida, esa partida se reinicia para el jugador restante.

## Cómo se juega

1. **Preparación.** Cada jugador coloca en secreto su flota de 5 barcos (17 casillas):
   portaaviones (5), acorazado (4), crucero (3), submarino (3) y destructor (2).
   Los barcos van horizontales o verticales, sin superponerse ni salirse del tablero
   de 10×10. Al confirmar ambos, empieza la partida.
2. **Turnos.** El servidor indica de quién es el turno. El jugador dispara a una
   casilla del tablero enemigo y el servidor responde **agua**, **tocado** o **hundido**.
   El turno pasa al oponente.
3. **Victoria.** Gana el primero que hunde toda la flota enemiga.

## Pruebas

```bash
make test        # go test ./... + vitest
make build       # binario Go + build de producción del frontend
```

## Despliegue

El backend vive en **Cloud Run (GCP)** y el frontend en **Cloudflare Pages**.
El estado de la partida está en memoria, así que el backend corre con una sola
instancia (`--min-instances=1 --max-instances=1`) y afinidad de sesión: los dos
jugadores deben caer en la misma instancia.

### Backend → Cloud Run

```bash
export PATH="$HOME/google-cloud-sdk/bin:$PATH"

# Una sola vez: habilitar APIs y crear el repositorio de imágenes
gcloud services enable run.googleapis.com artifactregistry.googleapis.com \
  cloudbuild.googleapis.com --project=amarrete
gcloud artifacts repositories create midway \
  --repository-format=docker --location=us-central1 --project=amarrete

# Desplegar (el build ocurre en la nube, no hace falta Docker local)
cd apps/server
gcloud run deploy midway --source . \
  --image=us-central1-docker.pkg.dev/amarrete/midway/midway:v1 \
  --project=amarrete --region=us-central1 \
  --platform=managed --allow-unauthenticated \
  --min-instances=1 --max-instances=1 --session-affinity --port=8080
```

El servicio queda en dos URLs que apuntan al mismo servicio (una es el
alias de revisión corta y la otra la del número de proyecto):

- `https://midway-463140756941.us-central1.run.app` (la que reporta `gcloud`)
- `https://midway-akeen4rlgq-uc.a.run.app`

Compruébalo con `curl https://<servicio>/health`. Ambas enrutan al tráfico de
la última revisión, así que `wss://<cualquiera-de-las-dos>/ws` funciona. El
frontend se construye **después** del despliegue del backend, porque la URL del
WebSocket queda incrustada en el bundle: si cambia el backend, hay que
reconstruir y redesplegar el frontend.

### Frontend → Cloudflare Pages

La URL del WebSocket se embebe en el bundle en tiempo de build, así que el
frontend se construye **después** de desplegar el backend:

```bash
VITE_WS_URL="wss://midway-akeen4rlgq-uc.a.run.app/ws" \
  npm --workspace apps/midway run build

cd apps/midway && cf deploy dist
```

Queda en **https://midway.amarrete.com** (dominio propio en la zona
`amarrete.com`) y también en `https://midway.amarrete.workers.dev`.

La configuración está en `apps/midway/cloudflare.config.ts`. Ojo: el CLI `cf`
v1 **no** lee `wrangler.toml`; usa `cloudflare.config.ts`, y el subdominio va en
`worker.domains` (no en `routes`, que es lo que usa `wrangler` v3):

```ts
export default defineConfig({
  worker: {
    name: 'midway',
    compatibilityDate: '2026-11-13',
    assets: { notFoundHandling: 'single-page-application' },
    domains: [
      { hostname: 'midway.amarrete.com', production: true, previews: false },
    ],
  },
})
```

Además, el nombre del Worker lo deriva el CLI del `name` de `package.json`,
no del `name` del config: por eso el paquete se llama `midway` y no
`@midway/web`.

El archivo `cloudflare.config.ts` importa `defineConfig` de `@cloudflare/config`,
que es una devDependency normal de npm (`@cloudflare/config@^0.32.0`, la misma
versión que usa el CLI). Si el paquete no está instalado, el CLI no puede
leer el config y el deploy falla con `Cannot find package '@cloudflare/config'`.

### Notas
- El backend acepta cualquier `Origin` en el handshake WebSocket, necesario
  porque el frontend se sirve desde un dominio distinto al del backend.
- Cloud Run corta las conexiones WebSocket tras 60 minutos (límite de la
  plataforma); una partida más larga que eso se reconecta.
- `--min-instances=1` mantiene una instancia encendida, así que el servicio
  responde al instante y las partidas no se pierden al escalar a cero.
- El dominio `midway.amarrete.com` tarda unos minutos en responder la primera
  vez: Cloudflare emite el certificado TLS y propaga el DNS.
- Si `cf deploy` no aplica el dominio, comprueba que el `worker.config.json`
  generado en `.cloudflare/output/` incluye la clave `domains`.

## Variables de entorno

| Variable | Default | Descripción |
| --- | --- | --- |
| `PORT` | `8080` | Puerto del backend |
| `BOARD_SIZE` | `10` | Lado del tablero (columnas y filas) |
| `VITE_WS_URL` | *(derivada del host de la página)* | URL del WebSocket (frontend). Si no se define, se calcula como `ws://<host-de-la-página>:8080/ws`, lo que permite jugar en la red local sin configurarla. |

## Protocolo WebSocket

Mensajes JSON. Cliente → servidor:

| Mensaje | Campos | Descripción |
| --- | --- | --- |
| `create` | `name` | Crea una sala nueva y entra en ella (nombre opcional) |
| `join` | `name`, `room`, `key` | Entra a la sala existente con su código de 6 dígitos y su clave secreta |
| `leave` | — | Sale de la sala y vuelve al vestíbulo |
| `place` | `kind`, `x`, `y`, `orientation` | Coloca un barco en tu tablero |
| `remove` | `kind` | Quita un barco de tu tablero |
| `ready` | — | Confirma la flota (empieza cuando ambos confirman) |
| `shoot` | `x`, `y` | Dispara a una casilla del tablero enemigo |
| `start` | — | Reinicia la partida |
| `ping` | — | Keepalive |

Servidor → cliente:

| Mensaje | Campos | Descripción |
| --- | --- | --- |
| `welcome` | `playerId`, `room`, `key` | Confirmación de conexión; `room` es el código de 6 dígitos y `key` la clave secreta, que solo se envía al creador de la sala |
| `left` | — | Confirmación de salida de la sala |
| `state` | `state` | Instantánea de la partida (ver abajo) |
| `error` | `message` | Error de protocolo |
| `pong` | — | Respuesta a `ping` |

El `state` es una vista personalizada por jugador: `own` incluye tus barcos y los
disparos que recibiste; `enemy` oculta los barcos no hundidos y solo muestra los
disparos que hiciste.

```jsonc
{
  "type": "state",
  "state": {
    "phase": "setup",           // setup | playing | gameover
    "width": 10,
    "height": 10,
    "players": [{ "id": "…", "name": "Ana", "color": "#e94f6d", "ready": true }],
    "own":   { "ships": [], "sunkShips": [], "shots": [] },
    "enemy": { "ships": [], "sunkShips": [], "shots": [] },
    "currentTurn": "",
    "winner": "",
    "yourFleet":  [{ "kind": "carrier", "name": "Portaaviones", "size": 5 }],
    "enemyFleet": [{ "kind": "carrier", "name": "Portaaviones", "size": 5 }],
    "readyPlayers": ["…"]
  }
}
```

Tipos equivalentes en el frontend: `apps/web/src/game/types.ts`.
