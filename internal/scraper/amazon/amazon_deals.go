package amazon

import (
	"NovelScraper/internal/models"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/stealth"
)

type DepartmentOption struct {
	Value string
	Label string
}

type AmazonDealsScraper struct {
	Browser *rod.Browser
	BaseURL string
}

func NewAmazonDealsScraper(browser *rod.Browser, baseURL string) *AmazonDealsScraper {
	return &AmazonDealsScraper{Browser: browser, BaseURL: strings.TrimRight(baseURL, "/")}
}

// CollectDepartments opens /deals, expands Department list (See more), and returns value/label pairs.
func (s *AmazonDealsScraper) CollectDepartments() ([]DepartmentOption, error) {
	page, err := stealth.Page(s.Browser)
	if err != nil {
		return nil, err
	}
	defer page.MustClose()

	if err := page.Timeout(30 * time.Second).Navigate(s.BaseURL + "/deals"); err != nil {
		return nil, err
	}
	page.WaitLoad()
	// Attempt to click See more for departments
	if btn, err := page.Timeout(5 * time.Second).Element("button[aria-labelledby='see-more-departments-label']"); err == nil {
		_ = btn.Click("left", 1)
		page.Timeout(2 * time.Second).WaitStable(500 * time.Millisecond)
	}

	var options []DepartmentOption
	// Each department option is a div with data-testid starting with filter-departments- and contains an input[name='departments']
	elems, _ := page.Elements("div[data-a-input-name='departments']")
	for _, el := range elems {
		// input value
		val := ""
		if input, err := el.Element("input[name='departments']"); err == nil {
			v, _ := input.Attribute("value")
			if v != nil {
				val = *v
			}
		}
		if val == "" {
			continue
		}
		label := ""
		if lbl, err := el.Element("span.a-label .a-size-base"); err == nil {
			label = strings.TrimSpace(lbl.MustText())
		}
		if label == "" {
			// fallback: whole el text
			label = strings.TrimSpace(el.MustText())
		}
		options = append(options, DepartmentOption{Value: val, Label: label})
	}
	// Ensure "All" exists at index 0 if present
	if len(options) == 0 {
		return options, nil
	}
	return options, nil
}

// BuildDoubleEncodedDealsURL builds the deals URL with the provided filters and double URL-encodes the discounts-widget value.
func (s *AmazonDealsScraper) BuildDoubleEncodedDealsURL(departmentValue string, minPrice, maxPrice, minOff, maxOff int) (string, error) {
	// Define the structure for the JSON payload
	type priceRange struct {
		Min int `json:"min"`
		Max int `json:"max"`
	}
	type percentRange struct {
		Min int `json:"min"`
		Max int `json:"max"`
	}
	payload := map[string]interface{}{
		"state": map[string]interface{}{
			"refinementFilters": map[string]interface{}{
				"departments": []string{departmentValue},
			},
			"rangeRefinementFilters": map[string]interface{}{
				"price":      priceRange{Min: minPrice, Max: maxPrice},
				"percentOff": percentRange{Min: minOff, Max: maxOff},
			},
		},
		"version": 1,
	}

	// 1. Marshal the payload to a JSON string
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload to JSON: %w", err)
	}
	jsonString := string(jsonBytes)

	// 2. Wrap the JSON string in quotes. This is the crucial step.
	// The expected raw value is `"{...}"`, not `{...}`.
	quotedJSON := strconv.Quote(jsonString)

	// 3. URL-encode the result TWICE.
	encodedValue := url.QueryEscape(url.QueryEscape(quotedJSON))

	// 4. Construct the final URL.
	finalURL := fmt.Sprintf("%s/deals?discounts-widget=%s", s.BaseURL, encodedValue)

	return finalURL, nil
}

// humanlikeScroll scrolls the page down in small increments to reliably trigger lazy-loading content.
func humanlikeScroll(page *rod.Page) error {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	// Scroll down in small steps
	for i := 0; i < 10; i++ { // Perform up to 10 small scrolls to reach the bottom
		// Check if we are already at the bottom of the page
		isAtBottom, err := page.Eval(`() => window.innerHeight + window.pageYOffset >= document.body.scrollHeight - 10`)
		if err != nil {
			return err
		}
		if isAtBottom.Value.Bool() {
			break // Exit if already at the bottom
		}

		// Scroll down by a fraction of the window height
		_, err = page.Eval(`() => window.scrollBy(0, window.innerHeight * 0.5)`)
		if err != nil {
			return err
		}
		// Wait a short, random amount of time to appear more human
		time.Sleep(time.Duration(100+rand.Intn(150)) * time.Millisecond)
	}

	// "Wiggle" at the bottom to ensure all triggers fire
	_, _ = page.Eval(`() => window.scrollBy(0, -200)`)
	time.Sleep(200 * time.Millisecond)
	_, _ = page.Eval(`() => window.scrollBy(0, 400)`) // Scroll a bit further down just in case

	return nil
}

// ScrapeDealsGrid navigates to the encoded URL, scrolls/clicks load more until completion, and collects product links/titles/discounts.
func (s *AmazonDealsScraper) ScrapeDealsGrid(targetURL string) ([]models.Product, error) {
	page, err := stealth.Page(s.Browser)
	if err != nil {
		return nil, err
	}
	defer page.MustClose()

	if err := page.Timeout(40 * time.Second).Navigate(targetURL); err != nil {
		return nil, err
	}
	page.MustWaitLoad()
	log.Println("Successfully navigated to the filtered deals page.")

	seenProducts := make(map[string]bool)
	var products []models.Product
	discountRe := regexp.MustCompile(`(\d+)%`)

	stuckCounter := 0

	for i := 0; i < 100; i++ {
		// Use JavaScript to get the page's content height.
		previousHeightRes, err := page.Eval(`() => document.documentElement.scrollHeight`)
		if err != nil {
			log.Printf("Warning: could not get page height: %v", err)
			break
		}
		previousHeight := previousHeightRes.Value.Num()

		// Collect currently visible products
		cards, _ := page.Elements("div[data-testid='product-card']")
		newlyFoundCount := 0
		for _, card := range cards {
			linkEl, err := card.Element("a[data-testid='product-card-link']")
			if err != nil {
				continue
			}
			hrefAttr, _ := linkEl.Attribute("href")
			if hrefAttr == nil || *hrefAttr == "" {
				continue
			}

			href := *hrefAttr
			if seenProducts[href] {
				continue
			}

			seenProducts[href] = true
			newlyFoundCount++

			fullURL := href
			if !strings.HasPrefix(fullURL, "http") {
				fullURL = s.BaseURL + strings.TrimPrefix(fullURL, "/")
			}

			p := models.Product{ProductURL: fullURL, SourceSite: "amazon.ae"}

			if tEl, err := card.Element("p[id^='title-']"); err == nil {
				p.TitleEnglish = strings.TrimSpace(tEl.MustText())
			}

			if offEl, err := card.Element(".style_filledRoundedBadgeLabel__Vo-4g span"); err == nil {
				matches := discountRe.FindStringSubmatch(offEl.MustText())
				if len(matches) > 1 {
					if n, err := strconv.Atoi(matches[1]); err == nil {
						p.DiscountPercent = n
					}
				}
			}
			products = append(products, p)
		}

		if newlyFoundCount > 0 {
			log.Printf("Found %d new products. Total collected: %d", newlyFoundCount, len(products))
		}

		// Check the footer state to decide what to do next
		footer := page.MustElement("div[data-testid='load-more-footer']")

		// Condition 1: Scraping is finished
		if spacer, _ := footer.Element(".LoadMore-module__spacer_TKuLOc0qWGD0grhZzukT"); spacer != nil {
			log.Println("End of deals marker found. Scraping complete.")
			break
		}

		// Condition 2: "View more" button exists
		if viewMoreButton, err := footer.Element("button[data-testid='load-more-view-more-button']"); err == nil {
			log.Println("Clicking 'View more deals' button...")
			viewMoreButton.MustClick()

			// --- START: MODIFIED CODE ---
			// A more robust way to wait is to find a common loading element
			// and wait for it to disappear. Spinners often have a role='progressbar'.
			// First, wait for the progress bar to appear.
			loadingSpinner, err := page.Timeout(5 * time.Second).Element("[role='progressbar']")
			if err == nil {
				// If it appeared, now wait for it to become invisible.
				loadingSpinner.MustWaitInvisible()
			} else {
				// If it never appeared, maybe the content loaded instantly.
				// We can just wait a bit to be safe.
				page.Timeout(5 * time.Second).WaitStable(500 * time.Millisecond)
			}
			// --- END: MODIFIED CODE ---

			continue
		}

		// **USE THE NEW HUMAN-LIKE SCROLLING LOGIC**
		log.Println("Scrolling smoothly to load more...")
		if err := humanlikeScroll(page); err != nil {
			log.Printf("Error during scrolling: %v", err)
		}

		page.Timeout(10 * time.Second).WaitStable(1 * time.Second)

		// Check if the page is stuck
		newHeightRes, err := page.Eval(`() => document.documentElement.scrollHeight`)
		if err != nil {
			log.Printf("Warning: could not get page height after scroll: %v", err)
			break
		}
		newHeight := newHeightRes.Value.Num()

		if newHeight == previousHeight {
			stuckCounter++
			log.Printf("Page height has not changed. Stuck counter: %d", stuckCounter)
			if stuckCounter >= 3 {
				log.Println("Page seems to be stuck. Ending scrape.")
				break
			}
		} else {
			stuckCounter = 0
		}
	}

	return products, nil
}
