package main

import (
	"log"

	"github.com/QWiseDev/Img2Gallery/server-go/internal/config"
	"github.com/QWiseDev/Img2Gallery/server-go/internal/db"
)

func main() {
	cfg := config.Load()
	database, err := db.Open(cfg)
	if err != nil {
		log.Fatalf("database open failed: %v", err)
	}
	defer database.Close()
	if err := db.Upgrade(database, cfg); err != nil {
		log.Fatalf("database upgrade failed: %v", err)
	}
	log.Printf("database upgraded to schema version %d", db.CurrentSchemaVersion)
}
