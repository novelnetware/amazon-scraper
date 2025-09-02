package main

import (
	"NovelScraper/internal/server"
	"NovelScraper/internal/wpdatabase"
	"NovelScraper/pkg/config"
	"log"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// The server loads its own config
	cfg := config.LoadConfig("config.yml")

	// The server connects to its own dedicated database
	wpRepo := wpdatabase.InitDB("wordpress.db")
	defer wpRepo.Close()

	// Start the server with the config and the wordpress database
	log.Println("Starting WordPress API server...")
	server.Start(wpRepo, cfg)
}
