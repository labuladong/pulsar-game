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
	fmt.Println("input player name:")
	reader := bufio.NewReader(os.Stdin)
	name, _ := reader.ReadString('\n')
	if err := ebiten.RunGame(newGame(name, "/Users/labuladong/sndev-kjtest.json")); err != nil {
		log.Fatal(err)
	}
}
