package main

import (
	"bytes"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	raudio "github.com/hajimehoshi/ebiten/v2/examples/resources/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	log "github.com/sirupsen/logrus"
	"image/color"
	"math/rand"
	"strings"
	"time"
)

const (
	screenWidth        = 200
	screenHeight       = 200
	gridSize           = 20
	xGridCountInScreen = screenWidth / gridSize
	yGridCountInScreen = screenHeight / gridSize
	bombLength         = 8
	// there is total / obstacleRatio num obstacle in map
	obstacleRatio = 5
	// bomb explode after explodeTime second
	explodeTime = 2
	// flame disappear after flameTime second
	flameTime = 2
	// obstacle update every updateObstacleTime second
	updateObstacleTime = 3
	// random bomb appear every randomBombTime second
	randomBombTime = 2
)

type Game struct {
	// local player playerName
	localPlayerName string
	nameToPlayers   map[string]*playerInfo
	posToPlayers    map[Position]*playerInfo

	nameToBombs map[string]*Bomb
	posToBombs  map[Position]*Bomb

	flameMap map[Position]int

	obstacleMap map[Position]struct{}

	// audio player
	audioContext *audio.Context
	deadPlayer   *audio.Player

	// receive event to redraw our game
	eventCh chan Event
	// send local event to send to pulsar
	sendCh chan Event

	client *pulsarClient
}

func (g *Game) Close() {
	g.client.Close()
	close(g.sendCh)
	close(g.eventCh)
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
		//localPlayer.alive = false
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
				defer ticker.Stop()
				for i := 0; i < 8; i++ {
					select {
					case <-bomb.explodeCh:
						// bomb exploded, stop
						return
					case <-ticker.C:
						// todo why bomb can cross the obstacle?
						if _, ok = g.obstacleMap[nexPos]; !validCoordinate(nexPos) || ok {
							// move to border or obstacle, stop
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

	// set bomb on empty block
	if _, ok := g.posToBombs[localPlayer.pos]; !ok && bomb {
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

	for pos, _ := range g.posToBombs {
		ebitenutil.DrawRect(screen, float64(pos.X*gridSize), float64(pos.Y*gridSize), gridSize, gridSize, bombColor)
	}

	for pos, _ := range g.obstacleMap {
		ebitenutil.DrawRect(screen, float64(pos.X*gridSize), float64(pos.Y*gridSize), gridSize, gridSize, obstacleColor)
	}

	for _, player := range g.nameToPlayers {
		var userColor color.RGBA
		if player.alive {
			userColor = playerColor
		} else {
			userColor = deadPlayerColor
		}
		ebitenutil.DrawRect(screen, float64(player.pos.X*gridSize), float64(player.pos.Y*gridSize), gridSize, gridSize, userColor)
	}

	if !g.nameToPlayers[g.localPlayerName].alive {
		ebitenutil.DebugPrint(screen, fmt.Sprintf("You are dead, press R to revive."))
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

	// flames
	var positions []Position
	for i := pos.X - 1; i >= pos.X-bombLength; i-- {
		p := Position{X: i, Y: pos.Y}
		if _, ok := g.obstacleMap[p]; ok {
			break
		}
		positions = append(positions, p)
	}
	for i := pos.X; i <= pos.X+bombLength; i++ {
		p := Position{X: i, Y: pos.Y}
		if _, ok := g.obstacleMap[p]; ok {
			break
		}
		positions = append(positions, p)
	}
	for j := pos.Y - 1; j >= pos.Y-bombLength; j-- {
		p := Position{X: pos.X, Y: j}
		if _, ok := g.obstacleMap[p]; ok {
			break
		}
		positions = append(positions, p)
	}
	for j := pos.Y; j <= pos.Y+bombLength; j++ {
		p := Position{X: pos.X, Y: j}
		if _, ok := g.obstacleMap[p]; ok {
			break
		}
		positions = append(positions, p)
	}

	for _, position := range positions {
		if !validCoordinate(position) {
			continue
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
	var positions []Position
	for i := pos.X - bombLength; i < pos.X+bombLength+1; i++ {
		positions = append(positions, Position{X: i, Y: pos.Y})
	}
	for j := pos.Y - bombLength; j < pos.Y+bombLength+1; j++ {
		positions = append(positions, Position{X: pos.X, Y: j})
	}
	for _, position := range positions {
		if !validCoordinate(position) {
			continue
		}
		//if val, ok := g.flameMap[position]; !ok || val <= 0 {
		//	// the unexplode event only has position info,
		//	// so history event may trigger unexplode event unexpectedly,
		//	// so we ensure all grids is flame, then trigger this event
		//	return
		//}
		if val, ok := g.flameMap[position]; ok && val > 0 {
			g.flameMap[position] = val - 1
		} else if ok {
			g.flameMap[position] = 0
		}
	}
}

// produce a random bomb every second
func (g *Game) randomBombsEnable() {
	go func() {
		// every one seconds, generate a new bomb
		ticker := time.NewTicker(time.Second * randomBombTime)
		for {
			select {
			case <-ticker.C:
				randomPos := Position{
					X: rand.Intn(xGridCountInScreen),
					Y: rand.Intn(yGridCountInScreen),
				}
				if _, ok := g.obstacleMap[randomPos]; ok {
					continue
				}
				if _, ok := g.posToBombs[randomPos]; ok {
					continue
				}
				g.sendSync(&SetBombEvent{
					bombName: "random-" + randStringRunes(5),
					pos:      randomPos,
				})
			}
		}
	}()
}

// playerName will be the subscription name
// roomName will be the topic name
func newGame(playerName, roomName string) *Game {
	info := &playerInfo{
		name:   playerName,
		avatar: "fff",
		pos: Position{
			X: 0,
			Y: 0,
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
		client:          newPulsarClient(roomName, playerName),
	}

	// init audio player
	jabD, err := wav.DecodeWithoutResampling(bytes.NewReader(raudio.Jab_wav))
	g.audioContext = audio.NewContext(48000)
	g.deadPlayer, err = g.audioContext.NewPlayer(jabD)
	if err != nil {
		log.Fatal(err)
	}

	// init local player
	g.nameToPlayers[info.name] = info
	g.posToPlayers[info.pos] = info

	// use this channel to send to pulsar
	g.sendCh = make(chan Event, 20)
	// use this channel to receive from pulsar
	g.eventCh = g.client.start(g.sendCh)

	return g
}
