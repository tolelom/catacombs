package main

import (
	"log"
	"os"

	"github.com/tolelom/catacombs/game"
	"github.com/tolelom/catacombs/server"
	"github.com/tolelom/catacombs/store"
	"github.com/tolelom/catacombs/web"
)

func main() {
	os.MkdirAll("data", 0755)

	db, err := store.Open("data/catacombs.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	lobby := game.NewLobby()

	// Start web terminal server in background
	go func() {
		if err := web.Start(":8080", 2222); err != nil {
			log.Printf("Web server error: %v", err)
		}
	}()

	log.Println("Catacombs server starting — SSH :2222, Web :8080")
	if err := server.Start("0.0.0.0", 2222, lobby, db); err != nil {
		log.Fatal(err)
	}
}
