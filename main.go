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

	if err := ebiten.RunGame(newGame(playerName, "roomName", "/Users/labuladong/sndev-kjtest.json")); err != nil {
		log.Fatal(err)
	}
}
