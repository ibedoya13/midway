/**
 * Resolución de la URL del WebSocket.
 *
 * Para poder jugar desde varios dispositivos de la misma red (teléfono y
 * portátil), el frontend no puede apuntar a `localhost`: cada dispositivo
 * tiene que conectarse a la IP de la máquina que corre el servidor. Por eso
 * la URL se deriva del host desde el que se abrió la página, salvo que se
 * fije explícitamente con VITE_WS_URL.
 */

/** Puerto por defecto del backend Go. */
export const DEFAULT_BACKEND_PORT = 8080

/**
 * resolveWebSocketUrl returns the WebSocket URL to connect to.
 *
 * @param locationHost host of the page, e.g. "192.168.0.2:5173"
 * @param configuredUrl value of VITE_WS_URL, when it is set
 * @param backendPort port where the Go server listens
 */
export function resolveWebSocketUrl(
  locationHost: string,
  configuredUrl?: string,
  backendPort: number = DEFAULT_BACKEND_PORT,
): string {
  if (configuredUrl && configuredUrl.trim() !== '') {
    return configuredUrl.trim()
  }

  const hostname = locationHost.split(':')[0]
  if (!hostname || hostname === 'localhost' || hostname === '127.0.0.1') {
    return `ws://localhost:${backendPort}/ws`
  }
  return `ws://${hostname}:${backendPort}/ws`
}
