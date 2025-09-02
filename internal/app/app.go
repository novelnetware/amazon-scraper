package app

import (
	"NovelScraper/internal/database"
	"NovelScraper/internal/models"
	"NovelScraper/internal/scraper/amazon"
	"NovelScraper/internal/translator"
	"NovelScraper/internal/wpdatabase"
	"NovelScraper/pkg/config"
	"NovelScraper/utils"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-rod/rod/lib/launcher"

	"github.com/go-rod/rod"
)

// App is the main application structure holding all dependencies.
type App struct {
	Config *config.Config
	Repo   *database.DBRepository
}

// New creates a new application instance with all initial settings.
func New() *App {
	cfg := config.LoadConfig("config.yml")
	repo := database.InitDB("products.db")
	return &App{
		Config: cfg,
		Repo:   repo,
	}
}

// RunProductScraper now only orchestrates the product list scraping.
// All the Amazon-specific logic has been moved to the amazon package.
func (a *App) RunProductScraper() {
	log.Println("--- Starting Product List Scraping Task ---")

	// A browser is needed for the scraper to work with.
	u := launcher.New().Headless(a.Config.Scraper.Headless).MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	// 1. Create a new scraper instance for Amazon.
	amazonScraper := amazon.New(browser, a.Config.Scraper, a.Config.Amazon)

	// 2. Call the generic method to get the product list.
	productsToScrape, err := amazonScraper.ScrapeProductList()
	if err != nil {
		log.Fatalf("Failed to scrape product list: %v", err)
	}

	// 3. Save the collected products to the database.
	log.Printf("Collected %d product links. Saving to database...", len(productsToScrape))
	var savedCount int
	for _, p := range productsToScrape {
		if err := a.Repo.SaveProduct(p); err == nil {
			savedCount++
		}
	}
	log.Printf("Task finished. Successfully saved %d new products.", savedCount)
}

// RunDetailScraper scrapes details for products with status 'needs_details'.
func (a *App) RunDetailScraper() {
	log.Println("--- Starting Product Detail Scraping Task ---")

	productsToScrape, err := a.Repo.GetProductsForDetailScrape()
	if err != nil {
		log.Fatalf("Failed to get products for detail scraping: %v", err)
	}

	if len(productsToScrape) == 0 {
		log.Println("No products are awaiting detail scraping. Task finished.")
		return
	}
	log.Printf("Found %d products to scrape for details.", len(productsToScrape))

	numWorkers := utils.GetOptimalWorkerCount(a.Config.Scraper.Workers)
	jobs := make(chan models.Product, len(productsToScrape))
	results := make(chan models.Product, len(productsToScrape))
	const maxRetries = 3

	// Start workers
	for w := 1; w <= numWorkers; w++ {
		go func(workerID int) {
			workerLauncher := launcher.New().Headless(a.Config.Scraper.Headless).MustLaunch()
			workerBrowser := rod.New().ControlURL(workerLauncher).MustConnect()
			defer workerBrowser.MustClose()

			amazonScraper := amazon.New(workerBrowser, a.Config.Scraper, a.Config.Amazon)

			for product := range jobs {
				log.Printf("[Worker %d] Scraping details for: %s", workerID, product.ProductURL)
				var err error
				for attempt := 1; attempt <= maxRetries; attempt++ {
					err = amazonScraper.ScrapeProductDetails(&product)
					if err == nil {
						break
					}
					log.Printf("[Worker %d] Attempt %d failed for %s: %v", workerID, attempt, product.ProductURL, err)
					if attempt < maxRetries {
						time.Sleep(time.Duration(1000) * time.Millisecond)
					}
				}
				results <- product // Send to results channel regardless of success to unblock the main loop.
			}
		}(w)
	}

	// Send jobs
	for _, p := range productsToScrape {
		jobs <- p
	}
	close(jobs)

	// Collect results and update DB
	for i := 0; i < len(productsToScrape); i++ {
		detailedProduct := <-results
		if detailedProduct.TitleEnglish != "" { // A simple check for successful scrape
			if err := a.Repo.UpdateProductDetails(detailedProduct); err != nil {
				log.Printf("DB Update failed for %s: %v", detailedProduct.ProductURL, err)
			}
		}
	}
	log.Println("--- Product Detail Scraping Task Finished ---")
}

// RunTranslator fetches products needing translation and processes them using a fallback mechanism.
func (a *App) RunTranslator() {
	log.Println("--- Starting Smart Translation Task ---")

	// 1. Build an ordered list of translator clients
	var clients []translator.Translator
	providerMap := make(map[string]config.ProviderConfig) // Helper map
	for _, p := range a.Config.Translator.Providers {
		providerMap[p.Name] = p
	}

	// Add primary provider first
	if primaryConf, ok := providerMap[a.Config.Translator.PrimaryProvider]; ok {
		log.Printf("Primary provider set to: '%s'", primaryConf.Name)
		clients = append(clients, translator.NewOpenAICompatibleClient(primaryConf.ApiURL, primaryConf.ApiKey, primaryConf.Model))
	} else {
		log.Fatalf("Primary provider '%s' not found in config!", a.Config.Translator.PrimaryProvider)
	}

	// Add fallback providers in order
	for _, name := range a.Config.Translator.FallbackProviders {
		if fallbackConf, ok := providerMap[name]; ok {
			log.Printf("Fallback provider added: '%s'", fallbackConf.Name)
			clients = append(clients, translator.NewOpenAICompatibleClient(fallbackConf.ApiURL, fallbackConf.ApiKey, fallbackConf.Model))
		} else {
			log.Printf("Warning: Fallback provider '%s' not found in config, skipping.", name)
		}
	}

	// 2. Get products to translate
	products, err := a.Repo.GetProductsForTranslation()
	if err != nil {
		log.Fatalf("Failed to get products for translation: %v", err)
	}
	if len(products) == 0 {
		log.Println("No products are awaiting translation. Task finished.")
		return
	}
	log.Printf("Found %d products to translate.", len(products))

	// 3. Process each product
	for i, p := range products {
		log.Printf("Processing product [%d/%d]: %s", i+1, len(products), p.TitleEnglish)

		// -- Translate Title with Fallback --
		titlePrompt := fmt.Sprintf("Translate the following product title to simple and fluent Persian. Do not translate technical terms, brand names, or units like '4K', 'HD', '256GB', '5G'. Only translate the descriptive parts.\n\nTitle: \"%s\"", p.TitleEnglish)
		translatedTitle, err := tryTranslate(clients, titlePrompt, true)
		if err != nil {
			log.Printf("ERROR: All providers failed for title of product ID %d: %v. Setting status to 'translation_failed'.", p.ID, err)
			a.Repo.UpdateProductTranslation(p.ID, "", "", "translation_failed")
			continue // Move to the next product
		}

		// -- Translate Description with Fallback --
		descPrompt := fmt.Sprintf("Translate the following product description and specifications into fluent Persian. Keep the original HTML structure (like tables, lists, etc.) intact. Do not translate technical terms, brand names, or model numbers. Combine the description and specifications into a single, cohesive HTML block.\n\nDescription:\n%s\n\nSpecifications (HTML Table/List):\n%s", p.DescriptionEnglish, p.Specifications)
		translatedDesc, err := tryTranslate(clients, descPrompt, false) // Non-verbose for description
		if err != nil {
			log.Printf("ERROR: All providers failed for description of product ID %d: %v. Setting status to 'translation_failed'.", p.ID, err)
			a.Repo.UpdateProductTranslation(p.ID, translatedTitle, "", "translation_failed") // Save title at least
			continue
		}

		// 4. Update database on success
		log.Printf("Successfully translated product ID %d. Setting status to 'completed'.", p.ID)
		err = a.Repo.UpdateProductTranslation(p.ID, translatedTitle, translatedDesc, "completed")
		if err != nil {
			log.Printf("FATAL: Could not update database for product ID %d: %v", p.ID, err)
		}
	}

	log.Println("--- Smart Translation Task Finished ---")
}

// tryTranslate attempts to translate a prompt using a list of clients until one succeeds.
func tryTranslate(clients []translator.Translator, prompt string, verbose bool) (string, error) {
	var lastErr error
	for i, client := range clients {
		log.Printf("   - Attempting translation with provider #%d...", i+1)
		stream, err := client.TranslateStream(context.Background(), prompt)
		if err != nil {
			lastErr = err
			log.Printf("   - Provider #%d failed: %v", i+1, err)
			continue // Try next provider
		}

		var builder strings.Builder
		if verbose {
			fmt.Printf("   - Translated Text: ")
		}
		for chunk := range stream {
			if verbose {
				fmt.Print(chunk)
			}
			builder.WriteString(chunk)
		}
		if verbose {
			fmt.Println()
		}

		log.Printf("   - Provider #%d succeeded.", i+1)
		return builder.String(), nil // Success!
	}

	return "", fmt.Errorf("all providers failed. last error: %w", lastErr)
}

// RunAutomaticWorkflow executes the entire scraping and processing pipeline in sequence.
func (a *App) RunAutomaticWorkflow() {
	log.Println("====== STARTING AUTOMATIC WORKFLOW ======")

	log.Println("--- STEP 1 of 3: Scraping Product Deals ---")
	a.RunProductScraper()
	log.Println("--- STEP 1 of 3: COMPLETED ---")

	// A short pause between stages can be helpful
	time.Sleep(2 * time.Second)

	log.Println("--- STEP 2 of 3: Scraping Product Details ---")
	a.RunDetailScraper()
	log.Println("--- STEP 2 of 3: COMPLETED ---")

	time.Sleep(2 * time.Second)

	log.Println("--- STEP 3 of 3: Translating Product Content ---")
	a.RunTranslator()
	log.Println("--- STEP 3 of 3: COMPLETED ---")

	log.Println("====== AUTOMATIC WORKFLOW FINISHED SUCCESSFULLY ======")
}

// PublishCompletedProducts transfers translated data to the clean WordPress database.
func (a *App) PublishCompletedProducts() {
	log.Println("--- Starting Publishing Task ---")

	wpRepo := wpdatabase.InitDB("wordpress.db")
	defer wpRepo.Close()

	products, err := a.Repo.GetCompletedProducts()
	if err != nil {
		log.Fatalf("Failed to get completed products: %v", err)
	}
	if len(products) == 0 {
		log.Println("No new completed products to publish.")
		return
	}
	log.Printf("Found %d completed products to publish to wordpress.db.", len(products))

	var successCount int
	for _, p := range products {
		// --- Data Transformation ---
		// 1. Generate ASIN (as a string)
		asin := ""
		parts := strings.Split(p.ProductURL, "/")
		for i, part := range parts {
			if (part == "dp" || part == "gp") && i+1 < len(parts) {
				asin = strings.Split(parts[i+1], "?")[0]
				break
			}
		}

		// 2. Generate Slug (as a string)
		slug := utils.CreateSlug(p.TitleFarsi)

		// 3. Save to the clean database by passing the extra arguments.
		if err := wpRepo.SaveProduct(p, asin, slug); err != nil {
			log.Printf("Failed to save product %s to wordpress.db: %v", p.ProductURL, err)
			continue
		}

		// 4. (Optional) Update status in original database.
		// Note: p.ID is the original int64 database ID.
		if err := a.Repo.UpdateProductStatus(p.ID, "published"); err != nil {
			log.Printf("WARN: Failed to update status for product ID %d in source db: %v", p.ID, err)
		}
		successCount++
	}

	log.Printf("--- Publishing Task Finished. Successfully published %d products. ---", successCount)
}
