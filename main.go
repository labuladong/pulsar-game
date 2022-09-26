package main

import (
	"bufio"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"log"
	"os"
)

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Bomb man")
	//fmt.Println("input room name:")
	reader := bufio.NewReader(os.Stdin)
	//roomName, _ := reader.ReadString('\n')
	fmt.Println("input player name:")
	playerName, _ := reader.ReadString('\n')

	game := newGame(playerName, "roomName", "/Users/labuladong/Downloads/o-7udlj-free.json")
	game.randomBombs()
	defer game.Close()
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
