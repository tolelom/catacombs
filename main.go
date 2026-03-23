package main

import (
	"log"

	"github.com/tolelom/catacombs/server"
)

func main() {
	if err := server.Start("0.0.0.0", 2222); err != nil {
		log.Fatal(err)
	}
}
