/** GamePhase / helpers compartidos por la UI. */

/** COLUMN_LABELS are the column headers of the board, from A to J. */
export const COLUMN_LABELS = ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J']

/** BOARD_SIZE is the number of rows and columns of a board. */
export const BOARD_SIZE = 10

/**
 * cellLabel returns the human readable coordinate of a cell, like "E5".
 */
export function cellLabel(x: number, y: number): string {
  const column = COLUMN_LABELS[x] ?? '?'
  return `${column}${y + 1}`
}

/**
 * shipCells returns every cell occupied by a ship.
 */
export function shipCells(ship: { x: number; y: number; size: number; orientation: string }): {
  x: number
  y: number
}[] {
  const cells: { x: number; y: number }[] = []
  for (let i = 0; i < ship.size; i++) {
    if (ship.orientation === 'vertical') {
      cells.push({ x: ship.x, y: ship.y + i })
    } else {
      cells.push({ x: ship.x + i, y: ship.y })
    }
  }
  return cells
}

/**
 * shipsOverlap reports whether two ships share a cell.
 */
export function shipsOverlap(
  a: { x: number; y: number; size: number; orientation: string },
  b: { x: number; y: number; size: number; orientation: string },
): boolean {
  const cellsA = shipCells(a)
  return cellsA.some((cell) =>
    shipCells(b).some((other) => other.x === cell.x && other.y === cell.y),
  )
}
