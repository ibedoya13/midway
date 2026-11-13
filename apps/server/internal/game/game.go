package game

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// Player is a client in the match.
type Player struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`

	// ResumeToken identifies the player across a reconnection. It is never
	// broadcast: the snapshot builds its own Player values.
	ResumeToken string `json:"-"`

	// Disconnected is true while the player is away and waiting to come
	// back.
	Disconnected bool `json:"disconnected,omitempty"`

	// Board holds the player's own fleet and the shots it received.
	Board *Board `json:"-"`
	// Fleet is the set of ships the player still has to place.
	Fleet []ShipSpec `json:"-"`
	// Ready reports whether the player finished placing its fleet.
	Ready bool `json:"ready"`
}

// ShipView is a ship as seen by a client.
type ShipView struct {
	Kind        ShipKind    `json:"kind"`
	Name        string      `json:"name"`
	Size        int         `json:"size"`
	X           int         `json:"x"`
	Y           int         `json:"y"`
	Orientation Orientation `json:"orientation"`
}

// ShotView is a fired shot as seen by a client.
type ShotView struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Result Result `json:"result"`
}

// BoardView is one side of a match as seen by a client.
type BoardView struct {
	Width     int        `json:"width"`
	Height    int        `json:"height"`
	Ships     []ShipView `json:"ships"`
	SunkShips []ShipView `json:"sunkShips"`
	Shots     []ShotView `json:"shots"`
}

// Snapshot is the state of a match from the point of view of one player.
type Snapshot struct {
	Phase        Phase      `json:"phase"`
	Width        int        `json:"width"`
	Height       int        `json:"height"`
	Players      []Player   `json:"players"`
	Own          BoardView  `json:"own"`
	Enemy        BoardView  `json:"enemy"`
	CurrentTurn  string     `json:"currentTurn"`
	Winner       string     `json:"winner"`
	YourFleet    []ShipSpec `json:"yourFleet"`
	EnemyFleet   []ShipSpec `json:"enemyFleet"`
	ReadyPlayers []string   `json:"readyPlayers"`
}

// Game owns the match state. All methods are safe for concurrent use.
type Game struct {
	mu sync.RWMutex

	width, height int

	players map[string]*Player
	order   []string

	phase       Phase
	currentTurn string
	winner      string
}

// New creates a match in the setup phase.
func New(width, height int) *Game {
	return &Game{
		width:   width,
		height:  height,
		players: make(map[string]*Player),
		phase:   PhaseSetup,
	}
}

// Phase returns the current phase of the match.
func (g *Game) Phase() Phase {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.phase
}

// CurrentTurn returns the id of the player whose turn it is.
func (g *Game) CurrentTurn() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.currentTurn
}

// Winner returns the id of the winning player, empty when there is none.
func (g *Game) Winner() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.winner
}

// Players returns the players in join order.
func (g *Game) Players() []*Player {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]*Player, 0, len(g.order))
	for _, id := range g.order {
		if p, ok := g.players[id]; ok {
			out = append(out, p)
		}
	}
	return out
}

// Player returns a player by id.
func (g *Game) Player(id string) *Player {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.players[id]
}

// MaxNameLength is the number of characters of a player name. The name is
// sent in every snapshot, so an unbounded one would be broadcast over and
// over. The frontend limits it too, but a direct WebSocket client does not
// go through it.
const MaxNameLength = 16

// AddPlayer joins a player to the match. It returns false when the match
// already has two players.
func (g *Game) AddPlayer(id, name string) (*Player, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.players) >= 2 {
		return nil, false
	}
	name = SanitizeName(name, len(g.players)+1)
	if name == "" {
		name = "Jugador " + strconv.Itoa(len(g.players)+1)
	}
	color := "#e94f6d"
	if len(g.players) == 1 {
		color = "#4f8ef7"
	}
	token, err := randomToken()
	if err != nil {
		return nil, false
	}
	p := &Player{
		ID:          id,
		Name:        name,
		Color:       color,
		ResumeToken: token,
		Board:       NewBoard(g.width, g.height),
		Fleet:       DefaultFleet(),
	}
	g.players[id] = p
	g.order = append(g.order, id)
	return p, true
}

// SetDisconnected marks a player as away or back.
func (g *Game) SetDisconnected(id string, disconnected bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if p, ok := g.players[id]; ok {
		p.Disconnected = disconnected
	}
}

// ResumePlayer returns the player that owns the given resume token. It only
// succeeds for a player that is away: a token of a connected player cannot
// be used to take its seat.
func (g *Game) ResumePlayer(token string) (*Player, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if token == "" {
		return nil, false
	}
	for _, id := range g.order {
		p, ok := g.players[id]
		if !ok || p.ResumeToken != token {
			continue
		}
		if !p.Disconnected {
			return nil, false
		}
		p.Disconnected = false
		return p, true
	}
	return nil, false
}

// SanitizeName trims a player name to the allowed length and removes the
// control characters. It returns an empty string when there is nothing
// usable left.
func SanitizeName(name string, fallback int) string {
	cleaned := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, strings.TrimSpace(name))

	runes := []rune(cleaned)
	if len(runes) > MaxNameLength {
		runes = runes[:MaxNameLength]
	}
	return strings.TrimSpace(string(runes))
}

// randomToken returns a random token to resume a match.
func randomToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// RemovePlayer removes a player from the match.
func (g *Game) RemovePlayer(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.players[id]; !ok {
		return
	}
	delete(g.players, id)
	for i, pid := range g.order {
		if pid == id {
			g.order = append(g.order[:i], g.order[i+1:]...)
			break
		}
	}
	if len(g.players) < 2 {
		g.phase = PhaseSetup
		g.currentTurn = ""
		g.winner = ""
	}
}

// PlaceShip places a ship on the player's own board.
func (g *Game) PlaceShip(id string, kind ShipKind, x, y int, o Orientation) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	p, ok := g.players[id]
	if !ok {
		return fmt.Errorf("jugador desconocido")
	}
	if g.phase != PhaseSetup {
		return fmt.Errorf("la partida ya empezó")
	}
	if p.Ready {
		return fmt.Errorf("ya confirmaste tu flota")
	}
	if err := p.Board.PlaceShip(kind, x, y, o); err != nil {
		return err
	}
	return nil
}

// RemoveShip removes a ship from the player's own board. Removing a ship
// cancels the confirmation of the fleet, so the player can place it again.
func (g *Game) RemoveShip(id string, kind ShipKind) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	p, ok := g.players[id]
	if !ok {
		return fmt.Errorf("jugador desconocido")
	}
	if g.phase != PhaseSetup {
		return fmt.Errorf("la partida ya empezó")
	}
	if err := p.Board.RemoveShip(kind); err != nil {
		return err
	}
	p.Ready = false
	return nil
}

// Ready marks the player's fleet as complete. The match starts once both
// players are ready.
func (g *Game) Ready(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	p, ok := g.players[id]
	if !ok {
		return fmt.Errorf("jugador desconocido")
	}
	if g.phase != PhaseSetup {
		return fmt.Errorf("la partida ya empezó")
	}
	if len(p.Board.PlacedShips()) != len(p.Fleet) {
		return fmt.Errorf("coloca todos los barcos antes de confirmar")
	}
	p.Ready = true

	// The match only starts when the two players have both confirmed their
	// fleet: a single ready player must keep waiting for the opponent.
	if len(g.players) < 2 {
		return nil
	}
	for _, other := range g.players {
		if !other.Ready {
			return nil
		}
	}
	g.phase = PhasePlaying
	g.currentTurn = g.order[0]
	g.winner = ""
	return nil
}

// Shoot fires at a cell of the opponent's board.
func (g *Game) Shoot(id string, x, y int) (Result, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.phase != PhasePlaying {
		return "", fmt.Errorf("la partida no está en juego")
	}
	if g.currentTurn != id {
		return "", fmt.Errorf("no es tu turno")
	}
	if _, ok := g.players[id]; !ok {
		return "", fmt.Errorf("jugador desconocido")
	}
	defender := g.opponentOf(id)
	if defender == nil {
		return "", fmt.Errorf("no hay oponente")
	}

	// The coordinates address the opponent's grid, so they are validated
	// against the defender's board.
	result, err := defender.Board.Shoot(x, y)
	if err != nil {
		return "", err
	}

	if defender.Board.AllSunk() {
		g.phase = PhaseGameOver
		g.winner = id
		return result, nil
	}

	g.currentTurn = defender.ID
	return result, nil
}

// Reset returns the match to the setup phase and clears the fleets.
func (g *Game) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.phase = PhaseSetup
	g.currentTurn = ""
	g.winner = ""
	for _, p := range g.players {
		p.Ready = false
		p.Board = NewBoard(g.width, g.height)
	}
}

// SnapshotFor returns the match state from the point of view of one player.
func (g *Game) SnapshotFor(id string) Snapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// A view for an id that is not in the match is empty: it must not carry
	// the names of the players nor the board of any of them.
	if _, ok := g.players[id]; !ok {
		return emptySnapshot(g.width, g.height)
	}

	players := make([]Player, 0, len(g.order))
	ready := make([]string, 0, len(g.order))
	for _, pid := range g.order {
		p, ok := g.players[pid]
		if !ok {
			continue
		}
		players = append(players, Player{
			ID:           p.ID,
			Name:         p.Name,
			Color:        p.Color,
			Ready:        p.Ready,
			Disconnected: p.Disconnected,
		})
		if p.Ready {
			ready = append(ready, p.ID)
		}
	}

	// The lists must be empty arrays, never null: the frontend iterates them
	// directly to draw the boards.
	view := BoardView{
		Width:     g.width,
		Height:    g.height,
		Ships:     []ShipView{},
		SunkShips: []ShipView{},
		Shots:     []ShotView{},
	}
	enemyView := BoardView{
		Width:     g.width,
		Height:    g.height,
		Ships:     []ShipView{},
		SunkShips: []ShipView{},
		Shots:     []ShotView{},
	}
	yourFleet := DefaultFleet()
	enemyFleet := DefaultFleet()

	if p, ok := g.players[id]; ok {
		view.Ships = shipViews(p.Board.PlacedShips())
		view.SunkShips = shipViews(p.Board.SunkShips())
		view.Shots = shotViews(p.Board.Shots())
		yourFleet = p.Fleet
	}
	if opponent := g.opponentOf(id); opponent != nil {
		enemyFleet = opponent.Fleet
		enemyView.SunkShips = shipViews(opponent.Board.SunkShips())
		enemyView.Shots = shotViews(opponent.Board.Shots())
	}

	return Snapshot{
		Phase:        g.phase,
		Width:        g.width,
		Height:       g.height,
		Players:      players,
		Own:          view,
		Enemy:        enemyView,
		CurrentTurn:  g.currentTurn,
		Winner:       g.winner,
		YourFleet:    yourFleet,
		EnemyFleet:   enemyFleet,
		ReadyPlayers: ready,
	}
}

// emptySnapshot builds a snapshot without any information about the match.
func emptySnapshot(width, height int) Snapshot {
	return Snapshot{
		Phase:   PhaseSetup,
		Width:   width,
		Height:  height,
		Players: []Player{},
		Own: BoardView{
			Width: width, Height: height,
			Ships: []ShipView{}, SunkShips: []ShipView{}, Shots: []ShotView{},
		},
		Enemy: BoardView{
			Width: width, Height: height,
			Ships: []ShipView{}, SunkShips: []ShipView{}, Shots: []ShotView{},
		},
		CurrentTurn:  "",
		Winner:       "",
		YourFleet:    DefaultFleet(),
		EnemyFleet:   DefaultFleet(),
		ReadyPlayers: []string{},
	}
}

// opponentOf returns the other player of the match.
//
// An id that is not in the match has no opponent: falling back to the first
// player of the list would hand an outsider the board of a real player.
func (g *Game) opponentOf(id string) *Player {
	if _, ok := g.players[id]; !ok {
		return nil
	}
	for _, pid := range g.order {
		if pid == id {
			continue
		}
		if p, ok := g.players[pid]; ok {
			return p
		}
	}
	return nil
}

// shipViews converts ships to their client representation.
func shipViews(ships []Ship) []ShipView {
	out := make([]ShipView, 0, len(ships))
	for _, ship := range ships {
		out = append(out, ShipView{
			Kind:        ship.Kind,
			Name:        ship.Name,
			Size:        ship.Size,
			X:           ship.X,
			Y:           ship.Y,
			Orientation: ship.Orientation,
		})
	}
	return out
}

// shotViews converts a shot map to a sorted list.
func shotViews(shots map[Point]Result) []ShotView {
	out := make([]ShotView, 0, len(shots))
	for point, result := range shots {
		out = append(out, ShotView{X: point.X, Y: point.Y, Result: result})
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && (out[j].Y < out[j-1].Y || (out[j].Y == out[j-1].Y && out[j].X < out[j-1].X)); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// ParseCoordinate converts a coordinate like "E5" into board coordinates.
// It returns false when the coordinate is malformed or out of range.
func ParseCoordinate(input string) (int, int, bool) {
	text := strings.ToUpper(strings.TrimSpace(input))
	if len(text) < 2 {
		return 0, 0, false
	}
	column := text[0]
	if column < 'A' || column > 'J' {
		return 0, 0, false
	}
	row, err := strconv.Atoi(text[1:])
	if err != nil || row < 1 || row > 10 {
		return 0, 0, false
	}
	return int(column - 'A'), row - 1, true
}

// FormatCoordinate converts board coordinates into the "E5" notation.
func FormatCoordinate(x, y int) string {
	return string(rune('A'+x)) + strconv.Itoa(y+1)
}
