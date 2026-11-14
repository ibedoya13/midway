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

### Dependencias y auditoría

`npm audit` debe dar **0 vulnerabilidades**. Las que no tienen corrección
arriba (fix upstream) se resuelven con `overrides` en el `package.json` raíz,
limitados al rango vulnerable para no romper a los paquetes que sí dependen de
una versión antigua:

```json
"overrides": {
  "minimatch@>=7.0.0 <11.0.1": "^11.0.8",
  "glob@>=9.0.0 <14.0.2": "^14.0.6"
}
```

**Al cambiar un `overrides` hay que borrar el lockfile oculto**, o npm no
re-resuelve el árbol y aplica el cambio a medias:

```bash
rm -rf node_modules package-lock.json && npm install
```

Borrar solo `package-lock.json` no basta: npm usa
`node_modules/.package-lock.json` como caché y da la instalación por buena.


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
  npm --workspace apps/midway run build:prod

cd apps/midway && cf deploy dist
```

Usa `build:prod` y no `build`: la URL del WebSocket queda incrustada en el
bundle, así que un build hecho sin `VITE_WS_URL` produce un cliente que no
llega al servidor. `build:prod` comprueba el bundle después de construirlo y
falla si la URL no es la esperada. **Un `npm run build` a secas sobrescribe
`dist/` con un bundle inservible** (se queda apuntando a la red local), así que
si se usa, hay que reconstruir con la variable antes de desplegar.

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
| `ALLOWED_ORIGINS` | `https://midway.amarrete.com,http://localhost:5173,http://127.0.0.1:5173` | Lista separada por comas de los orígenes que pueden abrir un WebSocket. Un origen ajeno recibe **403** antes del handshake. Los orígenes de IP privada en los puertos 5173/4173 se permiten siempre, para jugar en la red local. |
| `ALLOW_EMPTY_ORIGIN` | `true` | Si los clientes sin cabecera `Origin` (sondas, scripts, health checks) pueden conectar. |
| `MAX_CONNECTIONS_PER_IP` | `20` | Conexiones simultáneas por dirección. Una IP es toda una casa u oficina detrás del mismo router. |
| `MAX_CONNECTIONS` | `2100` | Conexiones simultáneas en total (dos por sala más margen). |
| `MAX_ROOMS` | `1000` | Salas simultáneas. El valor sale del perfilado de la instancia (ver abajo). |
| `GOMEMLIMIT` | *(sin valor)* | Límite de memoria del heap de Go. Conviene fijarlo por debajo del límite de la instancia para que el recolector trabaje antes de que la plataforma mate el proceso. |
| `MESSAGES_PER_SECOND` | `20` | Mensajes por segundo y socket. Al superarlo, el servidor cierra esa conexión. |
| `RECONNECT_GRACE_SECONDS` | `300` | Tiempo que un jugador conserva su asiento tras perder la conexión. Pasado ese plazo, la plaza se libera. |

### Seguridad

- **Origen restringido.** `ServeWS` valida el `Origin` antes del upgrade: un
  sitio ajeno recibe `403` en lugar de un socket que podría usar para ocupar
  salas o inundar el servidor.
- **Límites de ritmo.** Un token bucket por socket y un límite de conexiones
  por IP. La instancia está fijada a 1 porque el estado vive en memoria, así
  que agotarla tumbaría el juego de todos.
- **La clave de sala es la credencial.** El código de 6 dígitos es público y
  legible; la clave de 32 hex es lo que deja entrar. Un id desconocido recibe
  un snapshot vacío, nunca el tablero de un jugador real.
- **Cabeceras del SPA.** `apps/midway/public/_headers` define la CSP (con
  `connect-src` apuntando al backend **solo por `wss://`**), `nosniff`,
  `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`, HSTS y
  `Cross-Origin-Resource-Policy`. Vite lo copia a `dist/` y Cloudflare lo
  sirve. El dev server de Vite no aplica este archivo, así que el modo LAN
  sigue funcionando sin `ws://` en la CSP.
- **El nombre del jugador lo limita el servidor**, no solo el navegador. El
  frontend usa `maxlength=16`, pero un cliente WebSocket directo no pasa por
  ahí: un nombre enorme se difunde en cada snapshot (20 por segundo). El
  servidor lo recorta a 16 caracteres y le quita los caracteres de control.
- **Reconexión.** Cloud Run cierra las WebSocket a los 60 minutos, así que el
  cliente reconecta solo: hasta 8 intentos con espera creciente (1 s a 15 s).
  Cada jugador recibe un **token de reanudación** que conserva en el servidor:
  al volver toma su asiento con la misma flota y el mismo turno. El token solo
  sirve para un jugador desconectado — el de alguien conectado se rechaza — y
  nunca viaja en el estado broadcast. Si el jugador no vuelve en
  `RECONNECT_GRACE_SECONDS`, su plaza se libera y la partida se reinicia.

> **Nota al desplegar:** `gcloud` usa la coma como separador de sus
> argumentos `key=value`, así que un valor con comas se rompe. Por eso
> `ALLOWED_ORIGINS` acepta también `;` como separador:
> `--update-env-vars "ALLOWED_ORIGINS=https://midway.amarrete.com;http://localhost:5173"`.

### Capacidad (perfilado de la instancia)

La instancia de Cloud Run tiene **512 MiB**. El perfilado con WebSocket reales
sobre TLS (no solo el heap de Go, sino el RSS del proceso) da:

| Salas (2 jugadores cada una) | RSS | % de 512 MiB | Ciclo de broadcast |
| --- | --- | --- | --- |
| 50 | 21,9 MiB | 4,3% | — |
| 150 | 29,8 MiB | 5,8% | — |
| 300 | 42,5 MiB | 8,3% | — |
| 500 | 59,2 MiB | 11,6% | 1,2 ms (2,3%) |
| **1000** | **190,5 MiB** | **37,2%** | **7,5 ms (15%)** |

La regresión lineal da **85 KiB por sala** con un coste fijo de 17,6 MiB, y
predice los puntos medidos con menos del 1% de error.

El límite real **no es la memoria sino la CPU**: el broadcast serializa un
snapshot por cliente 20 veces por segundo, así que cada ciclo debe caber en
50 ms. Con 1000 salas y 2000 clientes el ciclo tarda 7,5 ms (15% del
presupuesto); con 2000 salas serían 18 ms (36%).

Por eso `MAX_ROOMS=1000` y `MAX_CONNECTIONS=2100` (dos jugadores por sala más
margen). Por encima de ~1500 salas la memoria seguiría cabiendo, pero el
broadcast se acercaría al 30% de CPU y el juego empezaría a notarse.

Las cifras se reproducen con los tests de medición de
`apps/server/internal/ws/memory_test.go`. Son pruebas de carga, así que solo
se ejecutan cuando se piden:

```bash
cd apps/server
MIDWAY_MEASURE=1 go test ./internal/ws/ -run TestValidateCapacity -v
```

Sin la variable `MIDWAY_MEASURE` los tests se saltan: abren miles de
conexiones y no son aptos para una máquina de desarrollo ni para CI.

### Verificación de la seguridad

Las medidas se comprueban contra el **despliegue**, no contra el código. Las
sondas viven fuera del repo (en el scratch) y hacen estas preguntas:

| Sonda | Resultado esperado |
| --- | --- |
| `Origin: https://attacker.example` | `403 Forbidden` |
| `Origin: https://midway.amarrete.com` | `101 Switching Protocols` |
| `Origin: http://localhost:5173` | `101` (desarrollo local) |
| Sin `Origin` | `101` (sondas y scripts) |
| Conexiones desde una misma IP | `429` al superar `MAX_CONNECTIONS_PER_IP` |
| Socket que inunda mensajes | el servidor lo cierra |
| Socket que nunca se une | `0` frames de estado, aunque haya partida en curso |
| `shoot` sin estar en una sala | error `debes entrar en una sala antes de jugar` |

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
