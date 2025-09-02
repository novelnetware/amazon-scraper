package server

import (
	"NovelScraper/internal/models"
	"NovelScraper/internal/wpdatabase"
	"NovelScraper/pkg/config"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"
)

// Start now accepts the WPRepository for the clean database.
func Start(repo *wpdatabase.WPRepository, cfg *config.Config) {
	http.HandleFunc("/products", productsHandler(repo))

	port := "8080"
	log.Printf("Starting API server on port %s", port)
	log.Println("Endpoint available at http://localhost:8080/products")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func productsHandler(repo *wpdatabase.WPRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Parse Pagination Parameters
		queryParams := r.URL.Query()
		page, _ := strconv.Atoi(queryParams.Get("page"))
		if page < 1 {
			page = 1
		}
		limit, _ := strconv.Atoi(queryParams.Get("limit"))
		if limit < 1 {
			limit = 20 // Default limit
		}
		offset := (page - 1) * limit

		filters := models.ProductFilters{Limit: limit, Offset: offset}

		// 2. Get Total Count for Pagination
		totalProducts, err := repo.CountProducts()
		if err != nil {
			http.Error(w, "Failed to count products", http.StatusInternalServerError)
			return
		}
		totalPages := int(math.Ceil(float64(totalProducts) / float64(limit)))

		// 3. Get Paginated Products
		products, err := repo.GetProducts(filters)
		if err != nil {
			http.Error(w, "Failed to get products", http.StatusInternalServerError)
			return
		}

		// 4. Build Final Response
		response := models.WordpressResponse{
			Data: products,
			Pagination: models.Pagination{
				TotalPages:  totalPages,
				CurrentPage: page,
			},
		}

		// 5. Send JSON Response with UTF-8 Header
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}
