package api2go

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// API is a REST JSONAPI.
type API struct {
	ContentType string
	*information
	resources []resource
}

// AddResource registers a data source for the given resource
// At least the CRUD interface must be implemented, all the other interfaces are
// optional. `resource` should be either an empty struct instance such as
// `Post{}` or a pointer to a struct such as `&Post{}`. The same type will be
// used for constructing new elements.
func (api *API) AddResource(rg *gin.RouterGroup, prototype Identifier,
	source CRUD) {
	api.addResource(rg, prototype, source)
}

// NewAPIWithResolver can be used to create an API with a custom URL resolver.
func NewAPI(prefix string, resolver URLResolver) *API {
	api := newAPI(prefix, resolver)
	return api
}

// newAPI is now an internal method that can be changed if params are changing
func newAPI(prefix string, resolver URLResolver) *API {
	// Add initial and trailing slash to prefix
	prefixSlashes := strings.Trim(prefix, "/")
	if len(prefixSlashes) > 0 {
		prefixSlashes = "/" + prefixSlashes + "/"
	} else {
		prefixSlashes = "/"
	}

	info := &information{prefix: prefixSlashes, resolver: resolver}

	api := &API{
		ContentType: defaultContentTypHeader,
		information: info,
	}

	return api
}
