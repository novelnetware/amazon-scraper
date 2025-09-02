package main

import (
	"NovelScraper/internal/app"
	"flag"
	"log"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	task := flag.String("task", "server", "Task to run: scrape-products, scrape-details, or server")
	flag.Parse()

	application := app.New()
	defer application.Repo.Close()

	log.Printf("Running task: %s", *task)

	switch *task {
	case "scrape-products":
		// This is Phase 1: Collects product links from the deals page.
		application.RunProductScraper()

	case "scrape-details":
		// This is Phase 2: Scrapes details for products collected in Phase 1.
		application.RunDetailScraper()

	case "translate":
		application.RunTranslator()

	case "publish":
		application.PublishCompletedProducts()

	case "automatic": // <-- ADD THIS NEW CASE
		application.RunAutomaticWorkflow()

	default:
		log.Fatalf("Unknown task: %s.", *task)
	}
}
