package amazon

import (
	"NovelScraper/internal/models"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// menuAPIResponse یک ساختار خصوصی برای آنمارشال کردن پاسخ JSON از API آمازون است.
type menuAPIResponse struct {
	Data string `json:"data"`
}

// fetchMenuHTML یک درخواست GET به URL داده شده ارسال کرده و HTML را از پاسخ JSON استخراج می‌کند.
func fetchMenuHTML(apiURL string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("could not read response body: %w", err)
	}

	var apiResp menuAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("could not unmarshal json: %w", err)
	}
	return apiResp.Data, nil
}

// parseCategoriesFromHTML یک رشته HTML را برای یافتن تمام لینک‌های دسته‌بندی و node های آنها پارس می‌کند.
func parseCategoriesFromHTML(htmlContent string) []models.Category {
	var categories []models.Category
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		log.Printf("Error parsing HTML: %v", err)
		return categories
	}

	var findNodeLinks func(*html.Node)
	findNodeLinks = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" && strings.Contains(a.Val, "node=") {
					parsedURL, _ := url.Parse(a.Val)
					nodeID := parsedURL.Query().Get("node")
					var categoryName string
					if n.FirstChild != nil {
						if n.FirstChild.Type == html.TextNode {
							categoryName = strings.TrimSpace(n.FirstChild.Data)
						} else if n.FirstChild.Type == html.ElementNode && n.FirstChild.FirstChild != nil {
							categoryName = strings.TrimSpace(n.FirstChild.FirstChild.Data)
						}
					}
					if nodeID != "" && categoryName != "" {
						categories = append(categories, models.Category{
							Name:       categoryName,
							Node:       nodeID,
							SourceSite: "amazon.ae",
						})
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findNodeLinks(c)
		}
	}

	findNodeLinks(doc)
	return categories
}

// ScrapeAllCategoriesDirectly تابع اصلی برای استخراج تمام دسته‌بندی‌ها با استفاده از فراخوانی مستقیم API است.
func ScrapeAllCategoriesDirectly() ([]models.Category, error) {
	log.Println("Scraper Module: Starting direct API calls...")

	firstLayerURL := "https://www.amazon.ae/nav/ajax/hMenuDesktopFirstLayer?ajaxTemplate=hMenuDesktopFirstLayer&pageType=NavUnknownPageType&hmDataAjaxHint=1&isFreshRegion=false&isFreshCustomer=false&isPrimeMember=false&isPrimeDay=false&isBackup=false&firstName=false&navDeviceType=desktop&hashCustomerAndSessionId=a1d46ae9e621de98031d7cf61b1533353571700b&environmentVFI=AmazonNavigationCards%2Fdevelopment%40B6360835559-AL2_aarch64&languageCode=en_AE&customerCountryCode=AE"
	mainContentURL := "https://www.amazon.ae/nav/ajax/hamburgerMainContent?ajaxTemplate=hamburgerMainContent&pageType=Gateway&hmDataAjaxHint=1&navDeviceType=desktop&isSmile=0&RegionalStores%5B%5D=Wf2HUUZ9yC&RegionalStores%5B%5D=hjCdvXXr28&isPrime=0&isBackup=false&hashCustomerAndSessionId=a1d46ae9e621de98031d7cf61b1533353571700b&languageCode=en_AE&environmentVFI=AmazonNavigationCards%2Fdevelopment%40B6360835559-AL2_aarch64&secondLayerTreeName=AmazonEcho%2Bkindle_books_reader%2Bamz_dev_home_sec%2Bmobile_tablets_acc%2Bcomp_office%2Btv_appl_elec%2Bsl_w_clothing_shoes%2Bsl_m_clothing_shoes%2Bsl_watch_bags_acc%2Bhealth_beauty_perfumes%2Bgrocery_gno%2Bhome_kitchen_pet%2Btools_home_improvement%2Btoys_babyproducts%2Bsports_outdoors%2Bbooks_gno%2Bvideo_games_gno%2Bautomotive%2Bamazon_global_store%2Bgno_homeservices%2Bglobal_exports&customerCountryCode=AE"

	// از map برای جلوگیری از ثبت دسته‌بندی تکراری استفاده می‌کنیم
	allCategoriesMap := make(map[string]models.Category)

	// مرحله ۱: دریافت و پارس منوی سطح اول
	log.Println("Scraper Module: Fetching first layer menu...")
	html1, err := fetchMenuHTML(firstLayerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch first layer menu: %w", err)
	}
	for _, cat := range parseCategoriesFromHTML(html1) {
		allCategoriesMap[cat.Node] = cat
	}

	// مرحله ۲: دریافت و پارس منوی اصلی (شامل تمام زیرمنوها)
	log.Println("Scraper Module: Fetching main content menu...")
	html2, err := fetchMenuHTML(mainContentURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch main content menu: %w", err)
	}
	for _, cat := range parseCategoriesFromHTML(html2) {
		allCategoriesMap[cat.Node] = cat
	}

	// تبدیل map به slice برای خروجی نهایی
	finalCategoryList := make([]models.Category, 0, len(allCategoriesMap))
	for _, cat := range allCategoriesMap {
		finalCategoryList = append(finalCategoryList, cat)
	}

	log.Printf("Scraper Module: Found %d unique categories.", len(finalCategoryList))
	return finalCategoryList, nil
}