package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"image/color"
	"strings"
	"time"
)

const (
	screenWidth        = 600
	screenHeight       = 500
	gridSize           = 10
	xGridCountInScreen = screenWidth / gridSize
	yGridCountInScreen = screenHeight / gridSize
	bombLength         = 8
)

type Game struct {
	// local player playerName
	localPlayerName string
	nameToPlayers   map[string]*playerInfo
	posToPlayers    map[Position]*playerInfo

	nameToBombs map[string]*Bomb
	posToBombs  map[Position]*Bomb

	flameMap map[Position]int

	// receive event to redraw our game
	eventCh chan Event
	// send local event to send to pulsar
	sendCh chan Event
	client *pulsarClient
}

func (g *Game) Update() error {
	// listen to event
	select {
	case event := <-g.eventCh:
		event.handle(g)
	default:
	}

	localPlayer := g.nameToPlayers[g.localPlayerName]

	info := &playerInfo{
		name:   localPlayer.name,
		pos:    localPlayer.pos,
		avatar: localPlayer.avatar,
		alive:  localPlayer.alive,
	}

	var dir = dirNone
	var bomb = false
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
		dir = dirLeft
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
		dir = dirRight
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		dir = dirDown
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		dir = dirUp
	} else if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		bomb = true
	} else if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		// revive
		event := &UserReviveEvent{
			playerInfo: info,
		}
		g.sendSync(event)
	} else if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		// quit game
	}

	if val, ok := g.flameMap[localPlayer.pos]; ok && val > 0 && localPlayer.alive {
		// dead due to boom
		event := &UserDeadEvent{
			playerInfo: info,
		}
		g.sendSync(event)
	}

	if dir != dirNone && localPlayer.alive {
		nexPos := getNextPosition(localPlayer.pos, dir)
		info.pos = nexPos
		event := &UserMoveEvent{
			playerInfo: info,
		}
		g.sendSync(event)
		if bomb, ok := g.posToBombs[nexPos]; ok {
			// push the bomb
			go func(bomb *Bomb, direction Direction) {
				nextPos := getNextPosition(bomb.pos, dir)
				ticker := time.NewTicker(time.Second / 2)
				for i := 0; i < 8; i++ {
					select {
					case <-bomb.explodeCh:
						// bomb exploded, stop
						return
					case <-ticker.C:
						if !validCoordinate(nextPos.X, nextPos.Y) {
							// move to border, stop
							return
						}
						event := &BombMoveEvent{
							bombName: bomb.bombName,
							pos:      nextPos,
						}
						g.sendSync(event)
						nextPos = getNextPosition(nextPos, dir)
					}
				}
			}(bomb, dir)
		}
	}

	if bomb {
		info.pos = localPlayer.pos
		event := &SetBombEvent{
			bombName: info.name + "-" + randStringRunes(5),
			pos:      info.pos,
		}
		g.sendSync(event)

	}

	return nil
}

// setBomb create a bomb with trigger channel
func (g *Game) setBombWithTrigger(bombName string, position Position, trigger chan struct{}) string {
	bomb := &Bomb{
		bombName:   bombName,
		playerName: strings.Split(bombName, "-")[0],
		pos:        position,
		explodeCh:  trigger,
	}
	g.nameToBombs[bomb.bombName] = bomb
	g.posToBombs[bomb.pos] = bomb
	return bomb.bombName
}

func (g *Game) removeBomb(bombName string) {
	if bomb, ok := g.nameToBombs[bombName]; ok {
		delete(g.nameToBombs, bombName)
		if _, ok = g.posToBombs[bomb.pos]; ok {
			delete(g.posToBombs, bomb.pos)
		}
	}
}

func (g *Game) sendSync(event Event) {
	// don't block
	select {
	case g.sendCh <- event:
	default:
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	// todo replace Rect with images
	for _, player := range g.nameToPlayers {
		var userColor color.RGBA
		if player.alive {
			userColor = playerColor
		} else {
			userColor = deadPlayerColor
		}
		ebitenutil.DrawRect(screen, float64(player.pos.X*gridSize), float64(player.pos.Y*gridSize), gridSize, gridSize, userColor)
	}

	for pos, _ := range g.posToBombs {
		ebitenutil.DrawRect(screen, float64(pos.X*gridSize), float64(pos.Y*gridSize), gridSize, gridSize, bombColor)
	}

	for pos, val := range g.flameMap {
		// only val > 0 means flame
		if val > 0 {
			ebitenutil.DrawRect(screen, float64(pos.X*gridSize), float64(pos.Y*gridSize), gridSize, gridSize, flameColor)
		}
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func (g *Game) explode(pos Position) {
	if bomb, ok := g.posToBombs[pos]; !ok {
		return
	} else {
		g.removeBomb(bomb.bombName)
	}
	x := pos.X
	y := pos.Y
	for i := pos.X - bombLength; i < pos.X+bombLength+1; i++ {
		position := Position{
			X: i,
			Y: y,
		}
		if val, ok := g.flameMap[position]; ok {
			g.flameMap[position] = val + 1
		} else {
			g.flameMap[position] = 1
		}
		// dead player
		if player, ok := g.posToPlayers[position]; ok {
			player.alive = false
		}
	}

	for j := pos.Y - bombLength; j < pos.Y+bombLength+1; j++ {
		position := Position{
			X: x,
			Y: j,
		}
		if val, ok := g.flameMap[position]; ok {
			g.flameMap[position] = val + 1
		} else {
			g.flameMap[position] = 1
		}
		// dead player
		if player, ok := g.posToPlayers[position]; ok {
			player.alive = false
		}
	}

}

func (g *Game) unExplode(pos Position) {
	x := pos.X
	y := pos.Y
	for i := pos.X - bombLength; i < pos.X+bombLength+1; i++ {
		if validCoordinate(i, y) {
			position := Position{
				X: i,
				Y: y,
			}
			if val, ok := g.flameMap[position]; ok {
				g.flameMap[position] = val - 1
			}
		}
	}

	for j := pos.Y - bombLength; j < pos.Y+bombLength+1; j++ {
		if validCoordinate(x, j) {
			position := Position{
				X: x,
				Y: j,
			}
			if val, ok := g.flameMap[position]; ok {
				g.flameMap[position] = val - 1
			}
		}
	}
}

func newGame(playerName, roomName, keyPath string) *Game {
	info := &playerInfo{
		name:   playerName,
		avatar: "fff",
		pos: Position{
			X: 10,
			Y: 20,
		},
		alive: true,
	}
	g := &Game{
		localPlayerName: playerName,
		nameToPlayers:   map[string]*playerInfo{},
		posToPlayers:    map[Position]*playerInfo{},
		nameToBombs:     map[string]*Bomb{},
		posToBombs:      map[Position]*Bomb{},
		flameMap:        map[Position]int{},
		eventCh:         nil,
		sendCh:          nil,
		client:          newPulsarClient("room-"+roomName, playerName, keyPath),
	}
	g.nameToPlayers[info.name] = info
	g.posToPlayers[info.pos] = info

	// use this channel to send to pulsar
	g.sendCh = make(chan Event, 20)
	// use this channel to receive from pulsar
	g.eventCh = g.client.start(g.sendCh)

	return g
}
