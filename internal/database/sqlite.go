package database

import (
	"NovelScraper/internal/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	_ "modernc.org/sqlite" // FIX: Switched to the pure Go driver
)

// DBRepository یک لایه دور کانکشن دیتابیس است.
type DBRepository struct {
	DB *sql.DB
}

// InitDB یک نمونه جدید از DBRepository را مقداردهی و برمی‌گرداند.
// FIX: نوع خروجی تابع به *DBRepository تغییر کرده است.
func InitDB(filepath string) *DBRepository {
	db, err := sql.Open("sqlite", filepath)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("Error pinging database: %v", err)
	}

	createProductsTableSQL := `
	CREATE TABLE IF NOT EXISTS products (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"source_site" TEXT,
		"product_url" TEXT UNIQUE,
		"category" TEXT,
		"status" TEXT DEFAULT 'new',
		"title_english" TEXT,
		"title_farsi" TEXT,
		"description_english" TEXT,
		"description_farsi" TEXT,
		"brand" TEXT,
		"availability" TEXT,
		"original_price" REAL,
		"discount_price" REAL,
		"discount_percent" INTEGER,
		"main_image_url" TEXT,
		"gallery_image_urls" TEXT,
		"specifications" TEXT,
		"country_of_origin" TEXT,
		"scraped_at" DATETIME,
		"posted_to_wp" BOOLEAN DEFAULT 0,
		"wp_post_id" INTEGER
	);`

	_, err = db.Exec(createProductsTableSQL)
	if err != nil {
		log.Fatalf("Error creating products table: %v", err)
	}

	// FIX: ایجاد جدول جدید برای دسته‌بندی‌ها
	createCategoriesTableSQL := `
	CREATE TABLE IF NOT EXISTS categories (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"name" TEXT,
		"node" TEXT UNIQUE,
		"source_site" TEXT
	);`
	_, err = db.Exec(createCategoriesTableSQL)
	if err != nil {
		log.Fatalf("Error creating categories table: %v", err)
	}

	log.Println("Database and tables initialized successfully.")
	return &DBRepository{DB: db}
}

// Close کانکشن دیتابیس را می‌بندد.
func (repo *DBRepository) Close() {
	repo.DB.Close()
}

// SaveProduct یک محصول را در دیتابیس ذخیره یا به‌روزرسانی می‌کند.
func (repo *DBRepository) SaveProduct(product models.Product) error {
	galleryJSON, err := json.Marshal(product.GalleryImageURLs)
	if err != nil {
		return err
	}

	query := `
	INSERT INTO products (
		source_site, product_url, category, status, title_english, brand, availability,
		original_price, discount_price, discount_percent, main_image_url,
		gallery_image_urls, specifications, description_english, scraped_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(product_url) DO UPDATE SET
		title_english=excluded.title_english,
		discount_percent=excluded.discount_percent,
		scraped_at=excluded.scraped_at;
	`
	// Note: We only update a few fields on conflict to avoid overwriting detailed data.
	// The status is only set on the initial insert.

	stmt, err := repo.DB.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		product.SourceSite, product.ProductURL, product.Category, "needs_details", // <-- Set category and initial status
		product.TitleEnglish, product.Brand, product.Availability,
		product.OriginalPrice, product.DiscountPrice, product.DiscountPercent, product.MainImageURL,
		string(galleryJSON), product.Specifications, product.DescriptionEnglish, time.Now(),
	)

	if err != nil {
		log.Printf("Failed to save product %s: %v", product.ProductURL, err)
		return err
	}

	// We can reduce log verbosity here
	log.Printf("Successfully saved product: %s", product.TitleEnglish)
	return nil
}

// GetAllProducts تمام محصولات ذخیره شده در دیتابیس را برمی‌گرداند.
func (repo *DBRepository) GetAllProducts() ([]models.Product, error) {
	rows, err := repo.DB.Query(`
		SELECT 
			product_url, title_farsi, brand, availability, original_price, 
			discount_price, discount_percent, main_image_url
		FROM products
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		// ستون‌های بیشتری را در صورت نیاز اضافه کنید
		if err := rows.Scan(
			&p.ProductURL, &p.TitleFarsi, &p.Brand, &p.Availability, &p.OriginalPrice,
			&p.DiscountPrice, &p.DiscountPercent, &p.MainImageURL,
		); err != nil {
			log.Printf("Error scanning product row: %v", err)
			continue
		}
		products = append(products, p)
	}
	return products, nil
}

// SaveCategory یک دسته‌بندی را در دیتابیس ذخیره یا جایگزین می‌کند.
func (repo *DBRepository) SaveCategory(category models.Category) error {
	query := `INSERT INTO categories (name, node, source_site) VALUES (?, ?, ?) ON CONFLICT(node) DO UPDATE SET name=excluded.name;`
	stmt, err := repo.DB.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(category.Name, category.Node, category.SourceSite)
	return err
}

// GetAllCategories تمام دسته‌بندی‌های یک سایت را از دیتابیس می‌خواند.
func (repo *DBRepository) GetAllCategories(sourceSite string) ([]models.Category, error) {
	rows, err := repo.DB.Query("SELECT id, name, node FROM categories WHERE source_site = ?", sourceSite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []models.Category
	for rows.Next() {
		var c models.Category
		c.SourceSite = sourceSite
		if err := rows.Scan(&c.ID, &c.Name, &c.Node); err != nil {
			continue
		}
		categories = append(categories, c)
	}
	return categories, nil
}

// GetIncompleteProducts retrieves products from the database that have not been fully scraped yet.
// We identify them as products where the brand is an empty string.
func (repo *DBRepository) GetIncompleteProducts() ([]models.Product, error) {
	rows, err := repo.DB.Query("SELECT id, product_url FROM products WHERE brand IS NULL OR brand = ''")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.ProductURL); err != nil {
			log.Printf("Error scanning incomplete product row: %v", err)
			continue
		}
		products = append(products, p)
	}
	return products, nil
}

// UpdateProductDetails updates an existing product record with fully scraped data.
func (repo *DBRepository) UpdateProductDetails(product models.Product) error {
	galleryJSON, err := json.Marshal(product.GalleryImageURLs)
	if err != nil {
		return err
	}

	query := `
	UPDATE products SET
		title_english = ?,
		brand = ?,
		availability = ?,
		original_price = ?,
		discount_price = ?,
		discount_percent = ?,
		main_image_url = ?,
		gallery_image_urls = ?,
		specifications = ?,
		description_english = ?,
		scraped_at = ?,
		status = ?       -- <-- ADD THIS LINE
	WHERE id = ?;
	`
	stmt, err := repo.DB.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		product.TitleEnglish,
		product.Brand,
		product.Availability,
		product.OriginalPrice,
		product.DiscountPrice,
		product.DiscountPercent, // Added discount percent
		product.MainImageURL,
		string(galleryJSON),
		product.Specifications,
		product.DescriptionEnglish,
		time.Now(),
		"needs_translation", // <-- Set status to the next step
		product.ID,
	)

	if err != nil {
		log.Printf("Failed to update product %d: %v", product.ID, err)
		return err
	}

	return nil
}

// GetProductsForTranslation retrieves all products with the status 'needs_translation'.
func (repo *DBRepository) GetProductsForTranslation() ([]models.Product, error) {
	rows, err := repo.DB.Query(`
		SELECT id, product_url, title_english, description_english, specifications
		FROM products
		WHERE status = 'needs_translation'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.ProductURL, &p.TitleEnglish, &p.DescriptionEnglish, &p.Specifications); err != nil {
			log.Printf("Error scanning product for translation: %v", err)
			continue
		}
		products = append(products, p)
	}
	return products, nil
}

// UpdateProductTranslation saves the translated text and updates the status.
func (repo *DBRepository) UpdateProductTranslation(id int64, titleFarsi, descriptionFarsi, newStatus string) error {
	query := `UPDATE products SET title_farsi = ?, description_farsi = ?, status = ? WHERE id = ?`
	stmt, err := repo.DB.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(titleFarsi, descriptionFarsi, newStatus, id)
	return err
}

// GetFilteredProducts retrieves products from the database based on a set of filters.
func (repo *DBRepository) GetFilteredProducts(filters models.ProductFilters) ([]models.Product, error) {
	var args []interface{}
	var conditions []string

	query := `SELECT id, source_site, product_url, category, status, title_farsi, 
	                 brand, availability, original_price, discount_price, 
	                 discount_percent, main_image_url 
	          FROM products WHERE 1=1`

	if filters.SourceSite != "" {
		conditions = append(conditions, "source_site = ?")
		args = append(args, filters.SourceSite)
	}
	if filters.Category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, filters.Category)
	}
	if filters.Availability != "" {
		conditions = append(conditions, "availability = ?")
		args = append(args, filters.Availability)
	}
	if filters.MinOriginalPrice > 0 {
		conditions = append(conditions, "original_price >= ?")
		args = append(args, filters.MinOriginalPrice)
	}
	if filters.MaxOriginalPrice > 0 {
		conditions = append(conditions, "original_price <= ?")
		args = append(args, filters.MaxOriginalPrice)
	}
	// Add more conditions for DiscountPrice and DiscountPercent if needed...

	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	// Add ordering and pagination
	query += " ORDER BY scraped_at DESC"
	if filters.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filters.Limit)
		if filters.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filters.Offset)
		}
	}

	rows, err := repo.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute filtered query: %w", err)
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(
			&p.ID, &p.SourceSite, &p.ProductURL, &p.Category, &p.Status, &p.TitleFarsi,
			&p.Brand, &p.Availability, &p.OriginalPrice, &p.DiscountPrice,
			&p.DiscountPercent, &p.MainImageURL,
		); err != nil {
			log.Printf("Error scanning filtered product row: %v", err)
			continue
		}
		products = append(products, p)
	}
	return products, nil
}

// GetProductsForDetailScrape retrieves products with the status 'needs_details'.
func (repo *DBRepository) GetProductsForDetailScrape() ([]models.Product, error) {
	rows, err := repo.DB.Query("SELECT id, product_url FROM products WHERE status = 'needs_details'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.ProductURL); err != nil {
			log.Printf("Error scanning incomplete product row: %v", err)
			continue
		}
		products = append(products, p)
	}
	return products, nil
}

// UpdateProductStatus changes the status of a product by its ID.
func (repo *DBRepository) UpdateProductStatus(id int64, newStatus string) error {
	_, err := repo.DB.Exec("UPDATE products SET status = ? WHERE id = ?", newStatus, id)
	return err
}

// GetCompletedProducts retrieves all products with the status 'completed'.
func (repo *DBRepository) GetCompletedProducts() ([]models.Product, error) {
	// Select all fields needed for the clean database
	rows, err := repo.DB.Query(`
		SELECT id, product_url, title_farsi, title_english, main_image_url, 
		original_price, discount_price, discount_percent, brand, availability,
		description_farsi, specifications
		FROM products
		WHERE status = 'completed'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		err := rows.Scan(&p.ID, &p.ProductURL, &p.TitleFarsi, &p.TitleEnglish, &p.MainImageURL,
			&p.OriginalPrice, &p.DiscountPrice, &p.DiscountPercent, &p.Brand, &p.Availability,
			&p.DescriptionFarsi, &p.Specifications)
		if err != nil {
			continue
		}
		products = append(products, p)
	}
	return products, nil
}
