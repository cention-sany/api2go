package api2go

import (
	"context"
)

// APIContexter embedding context.Context and requesting two helper functions
type APIContexter interface {
	context.Context
	Set(key string, value interface{})
	Get(key string) (interface{}, bool)
}
