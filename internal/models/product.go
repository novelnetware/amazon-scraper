package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// Product تمام اطلاعات استخراج شده برای یک محصول را نگهداری می‌کند.
type Product struct {
	ID                 int64           `db:"id"`
	SourceSite         string          `db:"source_site"`
	ProductURL         string          `db:"product_url"`
	Category           string          `db:"category"` // <-- ADD THIS LINE
	Status             string          `db:"status"`   // <-- ADD THIS LINE
	TitleEnglish       string          `db:"title_english"`
	TitleFarsi         string          `db:"title_farsi"`
	DescriptionEnglish string          `db:"description_english"`
	DescriptionFarsi   string          `db:"description_farsi"`
	Brand              string          `db:"brand"`
	Availability       string          `db:"availability"`
	OriginalPrice      float64         `db:"original_price"`
	DiscountPrice      float64         `db:"discount_price"`
	DiscountPercent    int             `db:"discount_percent"`
	MainImageURL       string          `db:"main_image_url"`
	GalleryImageURLs   JSONStringSlice `db:"gallery_image_urls"`
	Specifications     string          `db:"specifications"`
	CountryOfOrigin    string          `db:"country_of_origin"`
	ScrapedAt          time.Time       `db:"scraped_at"`
	PostedToWP         bool            `db:"posted_to_wp"`
	WPPostID           int             `db:"wp_post_id"`
}

// JSONStringSlice is a custom type to handle JSON serialization/deserialization for []string
type JSONStringSlice []string

// Value implements the driver.Valuer interface to convert []string to JSON for database storage
func (j JSONStringSlice) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface to convert JSON from database to []string
func (j *JSONStringSlice) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("unsupported type for JSONStringSlice")
	}
	return json.Unmarshal(bytes, j)
}

type Category struct {
	ID         int64
	Name       string // "Tablets"
	Node       string // "12050253031"
	Keyword    string // "Computer Tablets" - کلیدواژه برای ساخت URL جستجو
	SourceSite string
}

type Filters struct {
	MinPrice    int
	MaxPrice    int
	MinDiscount int
	// MaxDiscount حذف می‌شود چون در URL جدید استفاده نمی‌شود
}

type AmazonConfig struct {
	BaseURL            string
	DiscountsURLFormat string   // این دیگر استفاده نخواهد شد، اما فعلا نگه می‌داریم
	SearchURLFormat    string   // فرمت جدید برای URL جستجو
	Category           Category // به جای آرایه، یک دسته خاص را می‌گیرد
	Filters            Filters
}

// ProductFilters holds all possible query parameters for filtering products.
type ProductFilters struct {
	SourceSite         string
	Category           string
	Availability       string
	MinOriginalPrice   float64
	MaxOriginalPrice   float64
	MinDiscountPrice   float64
	MaxDiscountPrice   float64
	MinDiscountPercent int
	MaxDiscountPercent int
	// For Pagination
	Limit  int
	Offset int
}
