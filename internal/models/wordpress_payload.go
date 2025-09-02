package models

// WordpressResponse ساختار JSON است که سرور Go ما باید به وردپرس برگرداند.
type WordpressResponse struct {
	Data       []WordpressProduct `json:"data"`
	Pagination Pagination         `json:"pagination"`
}

type WordpressProduct struct {
	ID              string  `json:"id"` // ASIN محصول به عنوان ID
	Title           string  `json:"title"`
	Slug            string  `json:"slug"`
	ImageURL        string  `json:"image_url"`
	RealPrice       float64 `json:"real_price"`
	DiscountedPrice float64 `json:"discounted_price"`
	LinkOfProduct   string  `json:"link_of_product"`
}

type Pagination struct {
	TotalPages  int `json:"total_pages"`
	CurrentPage int `json:"current_page"`
}
