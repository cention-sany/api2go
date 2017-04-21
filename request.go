package api2go

// Request contains additional information for FindOne and Find Requests
type Request struct {
	QueryParams map[string][]string
	Pagination  map[string]string
	APIContexter
}
