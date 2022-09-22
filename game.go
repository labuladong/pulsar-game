package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
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
	// local player name
	localPlayerName string
	nameToPlays     map[string]*playerInfo
	posToBombs      map[Position]*Bomb
	flameMap        map[Position]struct{}

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
	} else if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {

	}

	localPlayer := g.nameToPlays[g.localPlayerName]

	info := &playerInfo{
		name:   localPlayer.name,
		avatar: localPlayer.avatar,
		alive:  localPlayer.alive,
	}

	if dir != dirNone && localPlayer.alive {
		info.pos = moveToNextPosition(localPlayer.pos, dir)
		event := &UserMoveEvent{
			playerInfo: info,
		}
		g.sendCh <- event
	}

	if bomb {
		info.pos = localPlayer.pos
		event := &SetBoomEvent{
			playerInfo: info,
		}
		g.sendCh <- event

		// explode after 2 seconds
		go func() {
			// bomb will explode after 2 seconds
			bombTimer := time.NewTimer(2 * time.Second)
			<-bombTimer.C
			g.sendCh <- &ExplodeEvent{
				pos: localPlayer.pos,
			}
			// explosion flame will disappear after 2 seconds
			flameTimer := time.NewTimer(2 * time.Second)
			<-flameTimer.C
			g.sendCh <- &UndoExplodeEvent{
				pos: localPlayer.pos,
			}
		}()
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	for _, player := range g.nameToPlays {
		ebitenutil.DrawRect(screen, float64(player.pos.X*gridSize), float64(player.pos.Y*gridSize), gridSize, gridSize, userColor)
	}

	for pos, _ := range g.posToBombs {
		ebitenutil.DrawRect(screen, float64(pos.X*gridSize), float64(pos.Y*gridSize), gridSize, gridSize, bombColor)
	}

	for pos, _ := range g.flameMap {
		ebitenutil.DrawRect(screen, float64(pos.X*gridSize), float64(pos.Y*gridSize), gridSize, gridSize, flameColor)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func (g *Game) explode(pos Position) {
	if _, ok := g.posToBombs[pos]; !ok {
		return
	}

	delete(g.posToBombs, pos)
	x := pos.X
	y := pos.Y
	for i := pos.X - bombLength; i < pos.X+bombLength+1; i++ {
		if validCoordinate(i, y) {
			g.flameMap[Position{
				X: i,
				Y: y,
			}] = struct{}{}
		}
	}
	for j := pos.Y - bombLength; j < pos.Y+bombLength+1; j++ {
		if validCoordinate(j, y) {
			g.flameMap[Position{
				X: x,
				Y: j,
			}] = struct{}{}
		}
	}

}

func (g *Game) unExplode(pos Position) {
	x := pos.X
	y := pos.Y
	for i := pos.X - bombLength; i < pos.X+bombLength+1; i++ {
		if validCoordinate(i, y) {
			delete(g.flameMap, Position{X: i, Y: y})
		}
	}
	for j := pos.Y - bombLength; j < pos.Y+bombLength+1; j++ {
		if validCoordinate(x, j) {
			delete(g.flameMap, Position{X: x, Y: j})
		}
	}
}

func newGame(name, keyPath string) *Game {
	info := &playerInfo{
		name:   name,
		avatar: "fff",
		pos: Position{
			X: 10,
			Y: 20,
		},
		alive: true,
	}
	g := &Game{
		localPlayerName: name,
		client:          newPulsarClient("test-11", name, keyPath),
		nameToPlays:     map[string]*playerInfo{},
		posToBombs:      map[Position]*Bomb{},
		flameMap:        map[Position]struct{}{},
	}
	g.nameToPlays[info.name] = info

	// use this channel to send to pulsar
	g.sendCh = make(chan Event, 20)
	// use this channel to receive from pulsar
	g.eventCh = g.client.start(g.sendCh)

	return g
}
