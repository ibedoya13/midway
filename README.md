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
| `join` | `name` | Entra a la sala (nombre opcional) |
| `place` | `kind`, `x`, `y`, `orientation` | Coloca un barco en tu tablero |
| `remove` | `kind` | Quita un barco de tu tablero |
| `ready` | — | Confirma la flota (empieza cuando ambos confirman) |
| `shoot` | `x`, `y` | Dispara a una casilla del tablero enemigo |
| `start` | — | Reinicia la partida |
| `ping` | — | Keepalive |

Servidor → cliente:

| Mensaje | Campos | Descripción |
| --- | --- | --- |
| `welcome` | `playerId`, `room` | Confirmación de conexión |
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
