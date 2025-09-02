package amazon

import (
	"NovelScraper/internal/models"
	"NovelScraper/utils"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// ImageInfo defines the structure for image data in the JSON
type ImageInfo struct {
	HiRes string           `json:"hiRes"`
	Thumb string           `json:"thumb"`
	Large string           `json:"large"`
	Main  map[string][]int `json:"main"`
}

// ScrapeProductDetails extracts all details from a single product page.
func ScrapeProductDetails(browser *rod.Browser, product *models.Product) error {
	if product.TitleEnglish != "" || !product.ScrapedAt.IsZero() {
		log.Printf("Product %s already scraped, skipping", product.ProductURL)
		return nil
	}

	log.Printf("Starting to scrape %s", product.ProductURL)
	page := browser.MustPage(product.ProductURL)
	defer page.MustClose()

	// Add random delay to avoid rate-limiting (1-3 seconds)
	time.Sleep(time.Duration(1000+rand.Intn(2000)) * time.Millisecond)

	// Wait for page to load with a 60-second timeout
	if err := page.Timeout(60 * time.Second).WaitLoad(); err != nil {
		log.Printf("Failed to wait for load: %v", err)
		return fmt.Errorf("failed to load page %s: %v", product.ProductURL, err)
	}
	log.Println("Page loaded successfully")

	// Check for robot check or CAPTCHA
	if has, _, err := page.Has("title"); has && err == nil {
		if title, err := page.Element("title"); err == nil {
			if titleText, err := title.Text(); err == nil {
				if strings.Contains(strings.ToLower(titleText), "robot check") || strings.Contains(strings.ToLower(titleText), "captcha") {
					log.Printf("Robot check or CAPTCHA detected in title: %s", titleText)
					return fmt.Errorf("robot check detected for %s", product.ProductURL)
				}
			}
		}
	}
	log.Println("No robot check in title")

	if err := handleCaptcha(page); err != nil {
		log.Printf("Captcha handling failed: %v", err)
		return fmt.Errorf("captcha handling failed for %s: %v", product.ProductURL, err)
	}
	log.Println("Captcha handling completed")

	// Wait for the main product container to be visible with fallback selectors
	el, err := page.Timeout(30 * time.Second).Element("#ppd")
	if err != nil {
		log.Printf("Primary container #ppd not found: %v", err)
		// Try fallback selectors
		alternativeSelectors := []string{"#dp", "#centerCol", "#productDetails"}
		for _, sel := range alternativeSelectors {
			el, err = page.Timeout(15 * time.Second).Element(sel)
			if err == nil {
				log.Printf("Found fallback container: %s", sel)
				break
			}
		}
		if err != nil {
			// Capture page HTML for debugging
			log.Printf("All container selectors failed: %v", err)
			return fmt.Errorf("no product container found for %s: %v", product.ProductURL, err)
		}
	}
	log.Println("Product container found")

	if err := el.WaitVisible(); err != nil {
		log.Printf("Product container not visible: %v", err)
		return fmt.Errorf("product container not visible for %s: %v", product.ProductURL, err)
	}
	log.Println("Product container is visible")

	// Scrape all data points using robust helper functions
	log.Println("Starting title extraction")
	product.TitleEnglish = extractTitle(page)
	log.Printf("Title extraction completed: %s", product.TitleEnglish)

	log.Println("Starting brand extraction")
	product.Brand = extractBrand(page)
	log.Printf("Brand extraction completed: %s", product.Brand)

	log.Println("Starting availability extraction")
	product.Availability = extractAvailability(page)
	log.Printf("Availability extraction completed: %s", product.Availability)

	log.Println("Starting price extraction")
	product.OriginalPrice, product.DiscountPrice, product.DiscountPercent = extractPrices(page)
	log.Printf("Price extraction completed: Original=%f, Discount=%f, Percent=%d", product.OriginalPrice, product.DiscountPrice, product.DiscountPercent)

	log.Println("Starting image extraction")
	product.MainImageURL, product.GalleryImageURLs = extractGallery(page)
	log.Printf("Image extraction completed: Main=%s, Gallery=%d images", product.MainImageURL, len(product.GalleryImageURLs))

	log.Println("Starting details extraction")
	product.Specifications, product.DescriptionEnglish = extractAllDetailsAsHTML(page)
	log.Printf("Details extraction completed: Specs=%d chars, Desc=%d chars", len(product.Specifications), len(product.DescriptionEnglish))

	product.ScrapedAt = time.Now()

	if product.TitleEnglish == "" {
		log.Println("No title extracted, scraping likely failed")
		return fmt.Errorf("failed to extract a title, scraping likely failed for %s", product.ProductURL)
	}

	log.Printf("Successfully scraped details for: %s", product.TitleEnglish)
	return nil
}

// --- Helper Functions ---

func handleCaptcha(page *rod.Page) error {
	hasCaptcha, _, err := page.Timeout(5 * time.Second).Has(`form[action="/errors/validateCaptcha"]`)
	if err != nil {
		log.Printf("Could not check for CAPTCHA form: %v", err)
		return fmt.Errorf("captcha check failed: %v", err)
	}
	if !hasCaptcha {
		log.Printf("No CAPTCHA form detected")
		return nil // No captcha
	}

	log.Println("!!! CAPTCHA page detected. Attempting to click 'Continue shopping'...")

	continueButton, err := page.Timeout(5 * time.Second).Element(`form[action="/errors/validateCaptcha"] button[type="submit"]`)
	if err != nil {
		log.Printf("Could not find the 'Continue shopping' button: %v", err)
		return fmt.Errorf("failed to find CAPTCHA button: %v", err)
	}

	if err := continueButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		log.Printf("Failed to click CAPTCHA button: %v", err)
		return fmt.Errorf("failed to click CAPTCHA button: %v", err)
	}

	// Wait for navigation after CAPTCHA
	wait := page.Timeout(10 * time.Second).WaitNavigation(proto.PageLifecycleEventNameLoad)
	wait()
	log.Println("... CAPTCHA likely solved.")

	// Verify page loaded correctly after CAPTCHA
	if has, _, err := page.Has("#ppd"); err != nil || !has {
		log.Printf("Product container not found after CAPTCHA navigation: %v", err)
		return fmt.Errorf("failed to load product page after CAPTCHA")
	}

	return nil
}

func extractTitle(page *rod.Page) string {
	if el, err := page.Timeout(10 * time.Second).Element("#productTitle"); err == nil {
		if title, err := el.Text(); err == nil {
			trimmedTitle := strings.TrimSpace(title)
			log.Printf("Extracted title: %s", trimmedTitle)
			return trimmedTitle
		} else {
			log.Printf("Failed to extract title text: %v", err)
		}
	}
	log.Println("Failed to extract title")
	return ""
}

func extractBrand(page *rod.Page) string {
	// Primary selector
	if el, err := page.Timeout(10 * time.Second).Element("#bylineInfo"); err == nil {
		if text, err := el.Text(); err == nil {
			text = strings.TrimPrefix(text, "Visit the ")
			text = strings.TrimPrefix(text, "Brand: ")
			text = strings.TrimSuffix(text, " Store")
			brand := strings.TrimSpace(text)
			log.Printf("Extracted brand: %s", brand)
			return brand
		}
	}
	// Fallback selector
	if el, err := page.Timeout(10 * time.Second).Element(".po-brand .po-break-word"); err == nil {
		if brand, err := el.Text(); err == nil {
			brand = strings.TrimSpace(brand)
			log.Printf("Extracted brand (fallback): %s", brand)
			return brand
		}
	}
	log.Println("Failed to extract brand")
	return ""
}

func extractAvailability(page *rod.Page) string {
	selectors := []string{
		"#availability",
		".a-section.a-spacing-none span.a-size-medium",
	}
	for _, selector := range selectors {
		if el, err := page.Timeout(10 * time.Second).Element(selector); err == nil {
			if availability, err := el.Text(); err == nil {
				availability = strings.TrimSpace(availability)
				if availability != "" {
					log.Printf("Extracted availability from %s: %s", selector, availability)
					return availability
				}
				log.Printf("Availability element %s found but empty", selector)
			}
		}
	}
	log.Println("Failed to extract availability from all selectors")
	return "Unknown"
}

func extractPrices(page *rod.Page) (original float64, discount float64, percent int) {
	// --- STRATEGY 1: Find the most common and reliable elements ---

	// Discount Price (the price the customer pays)
	// The most reliable element is usually the one with the class 'priceToPay'.
	if el, err := page.Element(".priceToPay .a-offscreen"); err == nil {
		if text, err := el.Text(); err == nil {
			discount = utils.ParsePrice(text)
		}
	}

	// Original Price (List Price)
	// The strikethrough price is the most reliable indicator of the original price.
	if el, err := page.Element("span[data-a-strike='true'] .a-offscreen"); err == nil {
		if text, err := el.Text(); err == nil {
			original = utils.ParsePrice(text)
		}
	}

	// Discount Percentage
	// The element with class '.savingsPercentage' is the most direct way to get this.
	if el, err := page.Element(".savingsPercentage"); err == nil {
		if text, err := el.Text(); err == nil {
			// Use a regex to find any number in the text (handles "-46%" and "46")
			re := regexp.MustCompile(`\d+`)
			pStr := re.FindString(text)
			if p, err := strconv.Atoi(pStr); err == nil {
				percent = p
			}
		}
	}

	// --- STRATEGY 2: Fallback to searching inside contextual text ---
	// This is useful if the primary selectors fail.

	// Search in spans that contain "List Price:"
	if original == 0.0 {
		if el, err := page.Element(`span.aok-offscreen:contains("List Price:")`); err == nil {
			if text, err := el.Text(); err == nil {
				original = utils.ParsePrice(text)
			}
		}
	}

	// Search in spans that contain "with...savings"
	if savingsEl, err := page.Element(`span.aok-offscreen:contains("percent savings")`); err == nil {
		if text, err := savingsEl.Text(); err == nil {
			// If we haven't found the discount price yet, try to get it from this text
			if discount == 0.0 {
				discount = utils.ParsePrice(text)
			}
			// If we haven't found the percentage yet, try to get it from this text
			if percent == 0 {
				re := regexp.MustCompile(`(\d+)\s*percent savings`)
				matches := re.FindStringSubmatch(text)
				if len(matches) > 1 {
					if p, err := strconv.Atoi(matches[1]); err == nil {
						percent = p
					}
				}
			}
		}
	}

	// --- STRATEGY 3: Final Sanity Checks and Calculations ---

	// If we still don't have an original price, it means the item is not on sale.
	// In this case, the original price is the same as the discount price.
	if original == 0.0 && discount > 0.0 {
		original = discount
	}

	// Final Fallback: If we have both prices but no percentage, calculate it.
	if percent == 0 && original > discount && discount > 0 {
		percent = int(((original - discount) / original) * 100)
		log.Println("Calculated discount percentage based on prices.")
	}

	log.Printf("Price extraction completed: Original=%.2f, Discount=%.2f, Percent=%d", original, discount, percent)
	return original, discount, percent
}

func extractGallery(page *rod.Page) (mainImage string, gallery []string) {
	seenImages := make(map[string]bool)
	var scriptContent string
	var found bool

	timeout := 15 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		scripts, err := page.Elements("script")
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		for _, script := range scripts {
			html, err := script.HTML()
			if err == nil && strings.Contains(html, "'colorImages'") {
				scriptContent = html
				found = true
				break
			}
		}
		if found {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !found {
		log.Println("Could not find the script with 'colorImages' within the timeout.")
		return "", nil
	}

	startMarker := "'initial':"
	startIndex := strings.Index(scriptContent, startMarker)
	if startIndex == -1 {
		log.Println("Could not find the 'initial': marker in the script tag.")
		return "", nil
	}

	jsonStartIndex := strings.Index(scriptContent[startIndex:], "[")
	if jsonStartIndex == -1 {
		log.Println("Could not find the opening bracket '[' for the image JSON array.")
		return "", nil
	}
	jsonStartIndex += startIndex

	balance := 0
	jsonEndIndex := -1
	for i := jsonStartIndex; i < len(scriptContent); i++ {
		switch scriptContent[i] {
		case '[':
			balance++
		case ']':
			balance--
			if balance == 0 {
				jsonEndIndex = i
				goto endLoop
			}
		}
	}
endLoop:

	if jsonEndIndex == -1 {
		log.Println("Could not find the matching closing bracket ']' for the image JSON array.")
		return "", nil
	}

	jsonArrayStr := scriptContent[jsonStartIndex : jsonEndIndex+1]
	log.Println("Successfully extracted image JSON array from script tag.")

	var images []ImageInfo
	if err := json.Unmarshal([]byte(jsonArrayStr), &images); err != nil {
		log.Printf("Failed to unmarshal image JSON data: %v", err)
		log.Printf("Problematic JSON string for debugging: %s", jsonArrayStr)
		return "", nil
	}

	// =================================================================
	// == START: NEW LOGIC FOR MAIN/GALLERY ACCORDING TO YOUR REQUIREMENT
	// =================================================================

	// 1. Check if any images were parsed at all.
	if len(images) == 0 {
		log.Println("JSON data was parsed, but the image array is empty.")
		return "", nil
	}

	// 2. The first image's 'hiRes' is ALWAYS the main image.
	if images[0].HiRes != "" {
		mainImage = images[0].HiRes
		// Add it to seenImages to avoid adding it to the gallery if it appears again.
		seenImages[mainImage] = true
	}

	// 3. Iterate through the REST of the images (from the second element onwards) for the gallery.
	if len(images) > 1 {
		// The slice images[1:] creates a new slice starting from the second element.
		for _, img := range images[1:] {
			if img.HiRes != "" && !seenImages[img.HiRes] {
				gallery = append(gallery, img.HiRes)
				seenImages[img.HiRes] = true
			}
		}
	}

	log.Printf("Successfully processed images: Main URL set, Gallery contains %d images", len(gallery))

	// =================================================================
	// == END: NEW LOGIC
	// =================================================================

	return mainImage, gallery
}

func extractAllDetailsAsHTML(page *rod.Page) (specifications string, description string) {
	var specBuilder, descBuilder strings.Builder

	// Product Overview (table - store as cleaned HTML)
	if el, err := page.Timeout(10 * time.Second).Element("#productOverview_feature_div table"); err == nil {
		if html, err := el.HTML(); err == nil {
			doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
			doc.Find("*").Each(func(i int, s *goquery.Selection) {
				for _, attr := range s.Nodes[0].Attr {
					s.RemoveAttr(attr.Key)
				}
			})
			cleanedHTML, _ := doc.Html()
			specBuilder.WriteString("<h2>Product Overview</h2>\n" + cleanedHTML)
			log.Println("Extracted and cleaned product overview table as HTML")
		}
	} else {
		log.Printf("Could not find product overview table: %v", err)
	}

	// Product Details (ul - store as raw text)
	if el, err := page.Timeout(10 * time.Second).Element("#detailBullets_feature_div"); err == nil {
		if text, err := el.Text(); err == nil {
			specBuilder.WriteString("<h2>Product Details</h2>\n" + strings.TrimSpace(text))
			log.Println("Extracted product details as raw text")
		}
	} else {
		log.Printf("Could not find product details list: %v", err)
	}

	// Product Information (expand and store table as cleaned HTML or text)
	if el, err := page.Timeout(10 * time.Second).Element("#prodDetails"); err == nil {
		if text, err := el.Text(); err == nil {
			specBuilder.WriteString("<h2>Product Information</h2>\n" + strings.TrimSpace(text))
			log.Println("Extracted product information as raw text")
		}
	} else {
		log.Printf("Could not find product information section: %v", err)
	}

	// About this item (extract plain text)
	if el, err := page.Timeout(10 * time.Second).Element("#feature-bullets"); err == nil {
		if text, err := el.Text(); err == nil {
			descBuilder.WriteString(strings.TrimSpace(text) + "\n")
			log.Println("Extracted feature bullets as plain text")
		}
	} else {
		log.Printf("Could not find feature bullets: %v", err)
	}

	// Product description (extract plain text)
	if el, err := page.Timeout(10 * time.Second).Element("#productDescription"); err == nil {
		if text, err := el.Text(); err == nil {
			descBuilder.WriteString(strings.TrimSpace(text))
			log.Println("Extracted product description as plain text")
		}
	} else {
		log.Printf("Could not find product description: %v", err)
	}

	return strings.TrimSpace(specBuilder.String()), strings.TrimSpace(descBuilder.String())
}
