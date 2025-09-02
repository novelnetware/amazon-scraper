package amazon

import (
	"NovelScraper/internal/models"
	"NovelScraper/pkg/config" // <-- Import the main config package

	"github.com/go-rod/rod"
)

// AmazonScraper now holds the correct, named config structs.
type AmazonScraper struct {
	Browser     *rod.Browser
	ScraperConf config.ScraperConfig
	AmazonConf  config.AmazonConfig
}

// New now accepts the specific config structs it needs.
func New(browser *rod.Browser, scraperConf config.ScraperConfig, amazonConf config.AmazonConfig) *AmazonScraper {
	return &AmazonScraper{
		Browser:     browser,
		ScraperConf: scraperConf,
		AmazonConf:  amazonConf,
	}
}

// ScrapeProductDetails remains the same.
func (s *AmazonScraper) ScrapeProductDetails(product *models.Product) error {
	return ScrapeProductDetails(s.Browser, product)
}
