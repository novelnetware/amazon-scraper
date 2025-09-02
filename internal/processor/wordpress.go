package processor

import (
	"NovelScraper/internal/models"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"
)

// WordpressClient برای ارتباط با API وردپرس استفاده می‌شود.
type WordpressClient struct {
	APIUrl     string
	Username   string
	Password   string
	HttpClient *http.Client
}

// TODO: این ساختار باید دقیقاً مطابق با نیاز Endpoint شما سفارشی‌سازی شود.
// این فقط یک نمونه اولیه است.
type WordpressPostPayload struct {
	Title         string   `json:"title"`
	EnglishTitle  string   `json:"english_title"`
	Content       string   `json:"content"`
	DetailsHTML   string   `json:"details_html"`
	OriginalPrice float64  `json:"original_price"`
	SalePrice     float64  `json:"sale_price"`
	Brand         string   `json:"brand"`
	ImageURLs     []string `json:"image_urls"`
	StockStatus   string   `json:"stock_status"`
	Status        string   `json:"status"` // e.g., "publish"
}

// NewWordpressClient یک کلاینت جدید برای وردپرس می‌سازد.
func NewWordpressClient(apiURL, username, password string) *WordpressClient {
	return &WordpressClient{
		APIUrl:     apiURL,
		Username:   username,
		Password:   password,
		HttpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// CreatePost یک محصول را به عنوان یک پست جدید در وردپرس ایجاد می‌کند.
func (c *WordpressClient) CreatePost(product models.Product) (string, error) {
	log.Printf("Preparing to send product to WordPress: %s", product.TitleEnglish)

	// ۱. تبدیل مدل محصول ما به ساختار JSON مورد نیاز وردپرس
	payload := WordpressPostPayload{
		Title:         product.TitleFarsi,
		EnglishTitle:  product.TitleEnglish,
		Content:       product.DescriptionFarsi,
		DetailsHTML:   product.Specifications,
		OriginalPrice: product.OriginalPrice,
		SalePrice:     product.DiscountPrice,
		Brand:         product.Brand,
		ImageURLs:     product.GalleryImageURLs,
		StockStatus:   product.Availability,
		Status:        "publish", // یا "draft" یا هر وضعیت دیگری
	}

	// ۲. تبدیل ساختار به داده JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	// ۳. ساخت درخواست HTTP
	req, err := http.NewRequest("POST", c.APIUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	// ۴. تنظیم هدرها و احراز هویت (Basic Auth)
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.Username, c.Password)

	// ۵. ارسال درخواست
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// ۶. بررسی پاسخ
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", errors.New("WordPress API returned an error. Status: " + resp.Status + " | Body: " + string(bodyBytes))
	}

	log.Printf("Successfully posted product to WordPress. Response: %s", string(bodyBytes))

	// میتوانید ID پست ایجاد شده را از پاسخ JSON استخراج و برگردانید.
	// برای مثال:
	// var responseMap map[string]interface{}
	// json.Unmarshal(bodyBytes, &responseMap)
	// postID := responseMap["id"]

	return string(bodyBytes), nil
}
