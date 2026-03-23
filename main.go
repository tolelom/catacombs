package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

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

	go func() {
		if err := server.Start("0.0.0.0", 2222, lobby, db); err != nil {
			log.Fatal(err)
		}
	}()

	log.Println("Catacombs server running on :2222")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Println("Shutting down...")
}
