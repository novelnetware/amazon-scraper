package wpdatabase

import (
	"NovelScraper/internal/models"
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

type WPRepository struct {
	DB *sql.DB
}

// InitDB creates and initializes the wordpress.db database.
func InitDB(filepath string) *WPRepository {
	db, err := sql.Open("sqlite", filepath)
	if err != nil {
		log.Fatalf("Error opening wordpress.db: %v", err)
	}

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS wp_products (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"product_url" TEXT UNIQUE,
		"asin" TEXT,
		"title_farsi" TEXT,
		"title_english" TEXT,
		"slug" TEXT,
		"image_url" TEXT,
		"original_price" REAL,
		"discount_price" REAL,
		"discount_percent" INTEGER,
		"brand" TEXT,
		"availability" TEXT,
		"description_farsi" TEXT,
		"specifications" TEXT
	);`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Error creating wp_products table: %v", err)
	}

	log.Println("wordpress.db and wp_products table initialized successfully.")
	return &WPRepository{DB: db}
}

func (repo *WPRepository) Close() {
	repo.DB.Close()
}

// SaveProduct inserts or updates a product in the clean database.
// SaveProduct now accepts the raw product and the generated asin and slug.
func (repo *WPRepository) SaveProduct(p models.Product, asin string, slug string) error {
	query := `
	INSERT INTO wp_products (
		product_url, asin, title_farsi, title_english, slug, image_url, 
		original_price, discount_price, discount_percent, brand, availability,
		description_farsi, specifications
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(product_url) DO UPDATE SET
		title_farsi=excluded.title_farsi,
		slug=excluded.slug,
		image_url=excluded.image_url,
		original_price=excluded.original_price,
		discount_price=excluded.discount_price,
		discount_percent=excluded.discount_percent;
	`
	// Use the passed-in asin and slug variables directly.
	_, err := repo.DB.Exec(query,
		p.ProductURL, asin, p.TitleFarsi, p.TitleEnglish, slug, p.MainImageURL,
		p.OriginalPrice, p.DiscountPrice, p.DiscountPercent, p.Brand, p.Availability,
		p.DescriptionFarsi, p.Specifications,
	)
	return err
}

// GetProducts retrieves a paginated list of products for the API.
func (repo *WPRepository) GetProducts(filters models.ProductFilters) ([]models.WordpressProduct, error) {
	// (This function can be enhanced with filters later if needed)
	query := "SELECT asin, title_farsi, slug, image_url, original_price, discount_price, product_url FROM wp_products ORDER BY id DESC LIMIT ? OFFSET ?"

	rows, err := repo.DB.Query(query, filters.Limit, filters.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []models.WordpressProduct
	for rows.Next() {
		var p models.WordpressProduct
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.ImageURL, &p.RealPrice, &p.DiscountedPrice, &p.LinkOfProduct); err != nil {
			continue
		}
		products = append(products, p)
	}
	return products, nil
}

// CountProducts returns the total number of products for pagination.
func (repo *WPRepository) CountProducts() (int, error) {
	var count int
	err := repo.DB.QueryRow("SELECT COUNT(*) FROM wp_products").Scan(&count)
	return count, err
}
