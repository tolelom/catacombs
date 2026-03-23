package main

import (
	"log"
	"os"

	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/server"
	"github.com/tolelom/catacombs/store"
)

func main() {
	os.MkdirAll("data", 0755)

	db, err := store.Open("data/catacombs.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	lobby := game.NewLobby()

	log.Println("Catacombs server starting on :2222")
	if err := server.Start("0.0.0.0", 2222, lobby, db); err != nil {
		log.Fatal(err)
	}
}
