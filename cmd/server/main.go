package main

import (
	"log"

	"steam-game-takeover-backend/internal/config"
	"steam-game-takeover-backend/internal/database"
	"steam-game-takeover-backend/internal/httpapi"
)

func main() {
	cfg := config.Load()

	db, err := database.Open(cfg.DBDSN)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	router := httpapi.NewRouter(cfg, db)
	log.Printf("server listening on %s", cfg.Addr)
	if err := router.Run(cfg.Addr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
