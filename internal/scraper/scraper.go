package scraper

import "NovelScraper/internal/models"

// Scraper defines the basic behavior for all website scrapers.
// It ensures that any new scraper we add (e.g., for Digikala)
// will follow a standard structure.
type Scraper interface {
	// ScrapeProductList scrapes the main listing page (like deals or search results)
	// and returns a slice of products with only basic info (URL, Title, etc.).
	ScrapeProductList() ([]models.Product, error)

	// ScrapeProductDetails takes a product with a URL and scrapes its detail page
	// to fill in all the other fields (Brand, Price, Description, etc.).
	ScrapeProductDetails(product *models.Product) error
}
