package main

import (
	"log"

	"github.com/sector-f/ssh-cli-forum/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
