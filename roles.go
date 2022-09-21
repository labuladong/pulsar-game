package main

import "image/color"

var (
	userColor  = color.RGBA{R: 0xFF, A: 0xff}
	bombColor  = color.RGBA{R: 0xab, A: 0xff}
	flameColor = color.RGBA{R: 0xab, A: 0xab}
)

type playerInfo struct {
	// localPlayer name
	name   string
	avatar string
	pos    Position
	alive  bool
}

type Direction int

const (
	dirNone Direction = iota
	dirLeft
	dirRight
	dirDown
	dirUp
)

func moveToNextPosition(position Position, direction Direction) Position {
	f := map[Direction]func(int, int) (int, int){
		dirLeft: func(x int, y int) (int, int) {
			return x - 1, y
		},
		dirRight: func(x int, y int) (int, int) {
			return x + 1, y
		},
		dirUp: func(x int, y int) (int, int) {
			return x, y - 1
		},
		dirDown: func(x int, y int) (int, int) {
			return x, y + 1
		},
		dirNone: func(x int, y int) (int, int) {
			return x, y
		},
	}
	x, y := f[direction](position.X, position.Y)
	res := Position{}
	if validCoordinate(x, y) {
		res.X = x
		res.Y = y
		return res
	}
	return position
}

func validCoordinate(x, y int) bool {
	return x >= 0 && y >= 0 && x < xGridCountInScreen && y < yGridCountInScreen
}

type Position struct {
	X int
	Y int
}

type Bomb struct {
	// the player name
	name string
	pos  Position
}
