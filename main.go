package main

import (
	"bufio"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"log"
	"os"
	"strings"
)

const privateKeyPath = ""
const pulsarUrl = ""

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Bomb man")
	fmt.Println("input room name:")
	reader := bufio.NewReader(os.Stdin)
	roomName, _ := reader.ReadString('\n')
	roomName = strings.Trim(roomName, "\n")

	fmt.Println("input player name:")
	playerName, _ := reader.ReadString('\n')
	playerName = strings.Trim(playerName, "\n")
	playerName = strings.ReplaceAll(playerName, "-", "_")

	game := newGame(playerName, roomName, privateKeyPath)
	game.randomBombs()
	defer game.Close()
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
