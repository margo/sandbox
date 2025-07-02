package northbound

// Pagination support
type PaginationRequest struct {
	Page     int `json:"page" query:"page" validate:"min=1"`
	PageSize int `json:"pageSize" query:"pageSize" validate:"min=1,max=100"`
}

type PaginationResponse struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalItems int `json:"totalItems"`
}
