package evotor

type Store struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Item struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Price         float64  `json:"price,omitempty"`
	Code          string   `json:"code,omitempty"`
	Barcodes      []string `json:"barcodes,omitempty"`
	ArticleNumber string   `json:"article_number,omitempty"`
	MeasureName   string   `json:"measure_name,omitempty"`
}

type DocumentPosition struct {
	ProductID   string  `json:"product_id,omitempty"`
	Name        string  `json:"name,omitempty"`
	ProductName string  `json:"product_name,omitempty"`
	Quantity    float64 `json:"quantity,omitempty"`
	Price       float64 `json:"price,omitempty"`
	Sum         float64 `json:"sum,omitempty"`
}

type DocumentBody struct {
	Positions []DocumentPosition `json:"positions,omitempty"`
	Sum       float64            `json:"sum,omitempty"`
	Total     float64            `json:"total,omitempty"`
}

type DocumentShort struct {
	ID        string       `json:"id"`
	Type      string       `json:"type"`
	CloseDate string       `json:"close_date"`
	DeviceID  string       `json:"device_id"`
	StoreID   string       `json:"store_id"`
	Body      DocumentBody `json:"body"`
	Total     float64      `json:"-"`
}

type DocumentFull struct {
	ID        string       `json:"id"`
	Type      string       `json:"type"`
	CloseDate string       `json:"close_date"`
	DeviceID  string       `json:"device_id"`
	StoreID   string       `json:"store_id"`
	Body      DocumentBody `json:"body"`
	Total     float64      `json:"-"`
}

type paging struct {
	NextCursor string `json:"next_cursor"`
}

type listResponse[T any] struct {
	Items  []T    `json:"items"`
	Paging paging `json:"paging"`
}

type SalesMetrics struct {
	Count         int            `json:"count"`
	TotalSum      float64        `json:"total_sum"`
	StoreID       string         `json:"store_id,omitempty"`
	From          string         `json:"from"`
	To            string         `json:"to"`
	DocumentTypes map[string]int `json:"document_types,omitempty"`
}
