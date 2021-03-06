package api2go

import "net/http"

// Request contains additional information for FindOne and Find Requests
type Request struct {
	QueryParams map[string][]string
	Pagination  map[string]string
	APIContexter
	*http.Request
}
