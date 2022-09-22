package main

import (
	"image/color"
	"math/rand"
)

var (
	playerColor     = color.RGBA{R: 0xFF, A: 0xff, G: 0x34}
	deadPlayerColor = color.RGBA{R: 0xeb, A: 0xc4, G: 0x40}
	bombColor       = color.RGBA{R: 0xab, A: 0xff, G: 0x3d}
	flameColor      = color.RGBA{R: 0xab, A: 0xab, G: 0xf4}
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

func getNextPosition(position Position, direction Direction) Position {
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
	playerName, bombName string
	pos                  Position
	// when exploded, this chanel will receive a message, control bomb moving
	explodeCh chan struct{}
}

func randStringRunes(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
