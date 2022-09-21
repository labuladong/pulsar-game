package main

const (
	UserMoveEventType    = "UserMoveEvent"
	UserJoinEventType    = "UserJoinEvent"
	SetBombEventType     = "SetBombEvent"
	ExplodeEventType     = "ExplodeEvent"
	UndoExplodeEventType = "UndoExplodeEvent"
)

// Event make change on Graph
type Event interface {
	handle(game *Game)
}

// UserMoveEvent makes playerInfo move
type UserMoveEvent struct {
	*playerInfo
}

func (a *UserMoveEvent) handle(g *Game) {
	newX, newY := a.pos.X, a.pos.Y
	if !validCoordinate(newX, newY) {
		// move out of boarder
		return
	}
	if player, ok := g.nameToPlays[a.name]; ok && !player.alive {
		// already dead
		return
	}
	g.nameToPlays[a.name] = a.playerInfo
}

type UserDeadEvent struct {
	*playerInfo
}

func (e *UserDeadEvent) handle(game *Game) {
	//TODO implement me
	panic("implement me")
}

type UserJoinEvent struct {
	*playerInfo
}

func (e *UserJoinEvent) handle(game *Game) {
	//TODO implement me
	panic("implement me")
}

type SetBoomEvent struct {
	*playerInfo
}

func (e *SetBoomEvent) handle(game *Game) {
	game.posToBombs[e.pos] = &Bomb{pos: e.pos, name: e.name}
}

type ExplodeEvent struct {
	name string
	pos  Position
}

func (e *ExplodeEvent) handle(game *Game) {
	game.explode(e.pos)
}

type UndoExplodeEvent struct {
	pos Position
}

func (e *UndoExplodeEvent) handle(game *Game) {
	game.unExplode(e.pos)
}
