package service

const (
	DefaultPageSize = 12
	MaxPageSize     = 100
)

type PageRequest struct {
	Page     int
	PageSize int
}

func (r PageRequest) Normalize() PageRequest {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 {
		r.PageSize = DefaultPageSize
	}
	if r.PageSize > MaxPageSize {
		r.PageSize = MaxPageSize
	}
	return r
}

func (r PageRequest) Offset() int {
	r = r.Normalize()
	return (r.Page - 1) * r.PageSize
}

type Page[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

func NewPage[T any](items []T, total int, request PageRequest) Page[T] {
	request = request.Normalize()
	if items == nil {
		items = make([]T, 0)
	}
	return Page[T]{Items: items, Total: total, Page: request.Page, PageSize: request.PageSize}
}
