package dto

type Device struct {
	DeviceName string  `json:"device_name"`
	Total      int     `json:"total"`
	Percentage float64 `json:"percentage"`
}
