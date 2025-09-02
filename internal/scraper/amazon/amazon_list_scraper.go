package amazon

import (
	"NovelScraper/internal/models"
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// ScrapeProductList handles the interactive process of scraping the Amazon deals page.
func (s *AmazonScraper) ScrapeProductList() ([]models.Product, error) {
	log.Println("Starting Amazon DEALS page scraping...")

	reader := bufio.NewReader(os.Stdin)

	tempLauncher := launcher.New().Headless(s.ScraperConf.Headless).MustLaunch()
	tempBrowser := rod.New().ControlURL(tempLauncher).MustConnect()

	dealsScraper := NewAmazonDealsScraper(tempBrowser, s.AmazonConf.BaseURL)
	departments, err := dealsScraper.CollectDepartments()
	tempBrowser.MustClose()

	if err != nil {
		return nil, fmt.Errorf("failed to collect departments: %w", err)
	}
	if len(departments) == 0 {
		return nil, fmt.Errorf("no departments found on deals page")
	}

	fmt.Println("Available departments:")
	for idx, d := range departments {
		fmt.Printf("[%d] %s\n", idx+1, d.Label)
	}
	fmt.Print("Choose a department index: ")
	depInput, _ := reader.ReadString('\n')
	depIdx, _ := strconv.Atoi(strings.TrimSpace(depInput))
	if depIdx < 1 || depIdx > len(departments) {
		depIdx = 1
	}
	chosenDept := departments[depIdx-1]

	fmt.Print("Enter min price: ")
	minPriceStr, _ := reader.ReadString('\n')
	fmt.Print("Enter max price: ")
	maxPriceStr, _ := reader.ReadString('\n')
	fmt.Print("Enter min percent off: ")
	minOffStr, _ := reader.ReadString('\n')
	fmt.Print("Enter max percent off: ")
	maxOffStr, _ := reader.ReadString('\n')
	minPrice, _ := strconv.Atoi(strings.TrimSpace(minPriceStr))
	maxPrice, _ := strconv.Atoi(strings.TrimSpace(maxPriceStr))
	minOff, _ := strconv.Atoi(strings.TrimSpace(minOffStr))
	maxOff, _ := strconv.Atoi(strings.TrimSpace(maxOffStr))

	dealsScraperForGrid := NewAmazonDealsScraper(s.Browser, s.AmazonConf.BaseURL)
	encodedURL, err := dealsScraperForGrid.BuildDoubleEncodedDealsURL(chosenDept.Value, minPrice, maxPrice, minOff, maxOff)
	if err != nil {
		return nil, fmt.Errorf("failed to build deals URL: %w", err)
	}
	log.Println("Constructed Target URL:", encodedURL)

	products, err := dealsScraperForGrid.ScrapeDealsGrid(encodedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape deals grid: %w", err)
	}

	for i := range products {
		products[i].Category = chosenDept.Label
		products[i].SourceSite = "amazon.ae"
	}

	return products, nil
}
