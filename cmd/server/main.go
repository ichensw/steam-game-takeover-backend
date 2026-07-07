package main

import (
	"context"
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
	if _, err := httpapi.EnsureBotQueryUser(db, cfg); err != nil {
		log.Fatalf("ensure bot query account: %v", err)
	}

	handler := httpapi.NewHandler(cfg, db)
	router := httpapi.NewRouter(handler)
	handler.StartTakeoverReminderWorker(context.Background())
	log.Printf("server listening on %s", cfg.Addr)
	if err := router.Run(cfg.Addr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
