package dto

type Paginate[T any] struct {
	Result []T `json:"result"`
	CurrentPage  int `json:"current_page"`
	LastPage int `json:"last_page"`
}
