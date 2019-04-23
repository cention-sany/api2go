package api2go

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/cention-sany/jsonapi"
	"github.com/gin-gonic/gin"
)

const (
	codeInvalidQueryFields  = "API2GO_INVALID_FIELD_QUERY_PARAM"
	defaultContentTypHeader = jsonapi.MediaType
)

var (
	queryPageRegex   = regexp.MustCompile(`^page\[(\w+)\]$`)
	queryFieldsRegex = regexp.MustCompile(`^fields\[(\w+)\]$`)
)

type information struct {
	prefix   string
	resolver URLResolver
}

func (i information) GetBaseURL() string {
	return i.resolver.GetBaseURL()
}

func (i information) GetPrefix() string {
	return i.prefix
}

type paginationQueryParams struct {
	number, size, offset, limit string
}

func newPaginationQueryParams(c *gin.Context) paginationQueryParams {
	var result paginationQueryParams
	result.number = c.Query(jsonapi.QueryParamPageNumber)
	result.size = c.Query(jsonapi.QueryParamPageSize)
	result.offset = c.Query(jsonapi.QueryParamPageOffset)
	result.limit = c.Query(jsonapi.QueryParamPageLimit)
	return result
}

func (p paginationQueryParams) isValid() bool {
	if p.number == "" && p.size == "" && p.offset == "" && p.limit == "" {
		return false
	}

	if p.number != "" && p.size != "" && p.offset == "" && p.limit == "" {
		return true
	}

	if p.number == "" && p.size == "" && p.offset != "" && p.limit != "" {
		return true
	}

	return false
}

func (p paginationQueryParams) getLinks(c *gin.Context, count uint,
	info information) (res *jsonapi.Links, err error) {
	result := make(jsonapi.Links)
	res = &result

	params := c.Request.URL.Query()
	prefix := ""
	baseURL := info.GetBaseURL()
	if baseURL != "" {
		prefix = baseURL
	}
	requestURL := fmt.Sprintf("%s%s", prefix, c.Request.URL.Path)

	if p.number != "" {
		// we have number & size params
		var number uint64
		number, err = strconv.ParseUint(p.number, 10, 64)
		if err != nil {
			return
		}

		if p.number != "1" {
			params.Set(jsonapi.QueryParamPageNumber, "1")
			query, _ := url.QueryUnescape(params.Encode())
			result["first"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s",
				requestURL, query)}

			params.Set(jsonapi.QueryParamPageNumber,
				strconv.FormatUint(number-1, 10))
			query, _ = url.QueryUnescape(params.Encode())
			result["prev"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s",
				requestURL, query)}
		}

		// calculate last page number
		var size uint64
		size, err = strconv.ParseUint(p.size, 10, 64)
		if err != nil {
			return
		}
		totalPages := (uint64(count) / size)
		if (uint64(count) % size) != 0 {
			// there is one more page with some len(items) < size
			totalPages++
		}

		if number != totalPages {
			params.Set(jsonapi.QueryParamPageNumber,
				strconv.FormatUint(number+1, 10))
			query, _ := url.QueryUnescape(params.Encode())
			result["next"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s", requestURL,
				query)}
			params.Set(jsonapi.QueryParamPageNumber,
				strconv.FormatUint(totalPages, 10))
			query, _ = url.QueryUnescape(params.Encode())
			result["last"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s", requestURL,
				query)}
		}
	} else {
		// we have offset & limit params
		var offset, limit uint64
		offset, err = strconv.ParseUint(p.offset, 10, 64)
		if err != nil {
			return
		}
		limit, err = strconv.ParseUint(p.limit, 10, 64)
		if err != nil {
			return
		}

		if p.offset != "0" {
			params.Set(jsonapi.QueryParamPageOffset, "0")
			query, _ := url.QueryUnescape(params.Encode())
			result["first"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s",
				requestURL, query)}
			var prevOffset uint64
			if limit > offset {
				prevOffset = 0
			} else {
				prevOffset = offset - limit
			}
			params.Set(jsonapi.QueryParamPageOffset,
				strconv.FormatUint(prevOffset, 10))
			query, _ = url.QueryUnescape(params.Encode())
			result["prev"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s", requestURL,
				query)}
		}

		// check if there are more entries to be loaded
		if (offset + limit) < uint64(count) {
			params.Set(jsonapi.QueryParamPageOffset,
				strconv.FormatUint(offset+limit, 10))
			query, _ := url.QueryUnescape(params.Encode())
			result["next"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s", requestURL,
				query)}

			params.Set(jsonapi.QueryParamPageOffset,
				strconv.FormatUint(uint64(count)-limit, 10))
			query, _ = url.QueryUnescape(params.Encode())
			result["last"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s", requestURL,
				query)}
		}
	}

	return
}

type resource struct {
	resourceType reflect.Type
	source       CRUD
	name         string
	api          *API
}

func (api *API) addResource(rg *gin.RouterGroup, prototype Identifier,
	source CRUD) *resource {
	resourceType := reflect.TypeOf(prototype)
	if resourceType.Kind() != reflect.Struct &&
		resourceType.Kind() != reflect.Ptr {
		panic("pass an empty resource struct or a struct pointer to AddResource!")
	}

	var ptrPrototype interface{}

	if resourceType.Kind() == reflect.Struct {
		ptrPrototype = reflect.New(resourceType).Interface()
	} else {
		ptrPrototype = reflect.ValueOf(prototype).Interface()
	}

	relation, err := findRelations(reflect.TypeOf(prototype), map[string]bool{})
	if err != nil {
		panic(fmt.Sprint("invalid node:", err))
	}
	name := relation.typ

	res := resource{
		resourceType: resourceType,
		name:         name,
		source:       source,
		api:          api,
	}

	requestInfo := func(c *gin.Context, api *API) *information {
		var info *information
		resolver, ok := api.information.resolver.(RequestAwareURLResolver)
		if ok {
			resolver.SetRequest(*c.Request)
			info = &information{
				prefix:   api.information.prefix,
				resolver: resolver,
			}
		} else {
			info = api.information
		}

		return info
	}

	baseURL := "/" + name

	rg.Handle("OPTIONS", baseURL, func(c *gin.Context) {
		c.Header("Allow", "GET,POST,PATCH,OPTIONS")
		c.Writer.WriteHeader(http.StatusNoContent)
	})

	rg.Handle("OPTIONS", baseURL+"/:id", func(c *gin.Context) {
		c.Header("Allow", "GET,PATCH,DELETE,OPTIONS")
		c.Writer.WriteHeader(http.StatusNoContent)
	})

	rg.Handle("GET", baseURL, func(c *gin.Context) {
		info := requestInfo(c, api)
		err := res.handleIndex(c, *info)
		if err != nil {
			api.handleError(err, c)
		}
	})

	rg.Handle("GET", baseURL+"/:id", func(c *gin.Context) {
		info := requestInfo(c, api)
		err := res.handleRead(c, *info)
		if err != nil {
			api.handleError(err, c)
		}
	})

	// generate all routes for linked relations if there are relations
	if len(relation.relations) > 0 {
		for _, rl := range relation.relations {
			rg.Handle("GET", baseURL+"/:id/relationships/"+rl.name, func(relation relationship) gin.HandlerFunc {
				return func(c *gin.Context) {
					info := requestInfo(c, api)
					err := res.handleReadRelation(c, *info, relation)
					if err != nil {
						api.handleError(err, c)
					}
				}
			}(*rl))

			rg.Handle("GET", baseURL+"/:id/"+rl.name, func(relation relationship) gin.HandlerFunc {
				return func(c *gin.Context) {
					info := requestInfo(c, api)
					err := res.handleLinked(c, api, relation, *info)
					if err != nil {
						api.handleError(err, c)
					}
				}
			}(*rl))

			rg.Handle("PATCH", baseURL+"/:id/relationships/"+rl.name, func(relation relationship) gin.HandlerFunc {
				return func(c *gin.Context) {
					err := res.handleReplaceRelation(c, relation)
					if err != nil {
						api.handleError(err, c)
					}
				}
			}(*rl))

			if _, ok := ptrPrototype.(EditToManyRelations); ok && rl.isMany {
				// generate additional routes to manipulate to-many relationships
				rg.Handle("POST", baseURL+"/:id/relationships/"+rl.name, func(relation relationship) gin.HandlerFunc {
					return func(c *gin.Context) {
						err := res.handleAddToManyRelation(c, relation)
						if err != nil {
							api.handleError(err, c)
						}
					}
				}(*rl))

				rg.Handle("DELETE", baseURL+"/:id/relationships/"+rl.name, func(relation relationship) gin.HandlerFunc {
					return func(c *gin.Context) {
						err := res.handleDeleteToManyRelation(c, relation)
						if err != nil {
							api.handleError(err, c)
						}
					}
				}(*rl))
			}
		}
	}

	rg.Handle("POST", baseURL, func(c *gin.Context) {
		info := requestInfo(c, api)
		err := res.handleCreate(c, info.prefix, *info)
		if err != nil {
			api.handleError(err, c)
		}
	})

	rg.Handle("DELETE", baseURL+"/:id", func(c *gin.Context) {
		err := res.handleDelete(c)
		if err != nil {
			api.handleError(err, c)
		}
	})

	rg.Handle("PATCH", baseURL+"/:id", func(c *gin.Context) {
		info := requestInfo(c, api)
		err := res.handleUpdate(c, *info)
		if err != nil {
			api.handleError(err, c)
		}
	})

	api.resources = append(api.resources, res)

	return &res
}

func buildReqParams(c *gin.Context) Request {
	req := Request{}
	params := make(map[string][]string)
	pagination := make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		params[key] = strings.Split(values[0], ",")
		pageMatches := queryPageRegex.FindStringSubmatch(key)
		if len(pageMatches) > 1 {
			pagination[pageMatches[1]] = values[0]
		}
	}
	req.Pagination = pagination
	req.QueryParams = params
	req.APIContexter = c
	req.Request = c.Request
	return req
}

func (res *resource) marshalResponse(c *gin.Context, rsp interface{},
	status int) error {
	filtered, err := filterSparseFields(rsp, c)
	if err != nil {
		return err
	}
	result, err := json.Marshal(filtered)
	if err != nil {
		return err
	}
	writeResult(c.Writer, result, status, res.api.ContentType)
	return nil
}

func (res *resource) handleIndex(c *gin.Context, info information) error {
	if source, ok := res.source.(PaginatedFindAll); ok {
		pagination := newPaginationQueryParams(c)

		if pagination.isValid() {
			count, response, err := source.PaginatedFindAll(buildReqParams(c))
			if err != nil {
				return err
			}

			paginationLinks, err := pagination.getLinks(c, count, info)
			if err != nil {
				return err
			}

			return res.respondWithPagination(c, response, info, http.StatusOK,
				paginationLinks)
		}
	}

	source, ok := res.source.(FindAll)
	if !ok {
		return NewHTTPError(nil, "Resource does not implement the FindAll interface", http.StatusNotFound)
	}

	response, err := source.FindAll(buildReqParams(c))
	if err != nil {
		return err
	}

	if response.StatusCode() == http.StatusAlreadyReported {
		return nil
	}
	return res.respondWith(c, response, info, http.StatusOK)
}

const idStr = "id"

func (res *resource) handleRead(c *gin.Context, info information) error {
	id := c.Param(idStr)
	response, err := res.source.FindOne(id, buildReqParams(c))
	if err != nil {
		return err
	}
	return res.respondWith(c, response, info, http.StatusOK)
}

func (res *resource) handleReadRelation(c *gin.Context, info information,
	relation relationship) error {
	id := c.Param(idStr)
	obj, err := res.source.FindOne(id, buildReqParams(c))
	if err != nil {
		return err
	}
	doc, err := marshalToDoc(obj.Result(), info)
	if err != nil {
		return err
	}
	node := doc.node()
	if node == nil {
		return NewHTTPError(nil,
			fmt.Sprintf("No node object nor relation %s", relation.name),
			http.StatusNotFound)
	}
	rel, ok := node.Relationships[relation.name]
	if !ok {
		return NewHTTPError(nil,
			fmt.Sprintf("There is no relation with the name %s", relation.name),
			http.StatusNotFound)
	}
	if metable, ok := obj.(Metable); ok {
		meta := metable.Metadata()
		if len(*meta) > 0 {
			doc.meta(meta)
		}
	}
	return res.marshalResponse(c, rel, http.StatusOK)
}

// try to find the referenced resource and call the findAll Method with referencing resource id as param
func (res *resource) handleLinked(c *gin.Context, api *API,
	linked relationship, info information) error {
	id := c.Param("id")
	for _, resource := range api.resources {
		if resource.name == linked.typ {
			request := buildReqParams(c)
			request.QueryParams[res.name+"ID"] = []string{id}
			request.QueryParams[res.name+"Name"] = []string{linked.name}

			if source, ok := resource.source.(PaginatedFindAll); ok {
				// check for pagination, otherwise normal FindAll
				pagination := newPaginationQueryParams(c)
				if pagination.isValid() {
					var count uint
					count, response, err := source.PaginatedFindAll(request)
					if err != nil {
						return err
					}

					paginationLinks, err := pagination.getLinks(c, count, info)
					if err != nil {
						return err
					}

					return res.respondWithPagination(c, response, info,
						http.StatusOK, paginationLinks)
				}
			}

			source, ok := resource.source.(FindAll)
			if !ok {
				return NewHTTPError(nil, "Resource does not implement the FindAll interface", http.StatusNotFound)
			}

			obj, err := source.FindAll(request)
			if err != nil {
				return err
			}
			return res.respondWith(c, obj, info, http.StatusOK)
		}
	}

	return NewHTTPError(
		errors.New("Not Found"),
		"No resource handler is registered to handle the linked resource "+linked.name,
		http.StatusNotFound,
	)
}

func (res *resource) handleCreate(c *gin.Context, prefix string,
	info information) error {
	// Ok this is weird again, but reflect.New produces a pointer, so we need
	// the pure type without pointer, otherwise we would have a pointer pointer
	// type that we don't want.
	resourceType := res.resourceType
	if resourceType.Kind() == reflect.Ptr {
		resourceType = resourceType.Elem()
	}
	newObj := reflect.New(resourceType).Interface()
	// Call InitializeObject if available to allow implementers change the
	// object before calling Unmarshal.
	if initSource, ok := res.source.(ObjectInitializer); ok {
		initSource.InitializeObject(newObj)
	}
	defer c.Request.Body.Close()
	err := jsonapi.UnmarshalPayload(c.Request.Body, newObj)
	if err != nil {
		return NewHTTPError(nil, err.Error(), http.StatusNotAcceptable)
	}
	var response Responder
	if res.resourceType.Kind() == reflect.Struct {
		// we have to dereference the pointer if user wants to use non pointer
		// values
		response, err = res.source.Create(reflect.ValueOf(newObj).Elem().Interface(),
			buildReqParams(c))
	} else {
		response, err = res.source.Create(newObj, buildReqParams(c))
	}
	if err != nil {
		return err
	}
	result, ok := response.Result().(Identifier)
	if !ok {
		return fmt.Errorf("Expected one newly created object by resource %s",
			res.name)
	}
	if len(prefix) > 0 {
		c.Header("Location", "/"+prefix+"/"+res.name+"/"+result.GetID())
	} else {
		c.Header("Location", "/"+res.name+"/"+result.GetID())
	}
	// handle 200 status codes
	code := response.StatusCode()
	switch code {
	case http.StatusCreated:
		return res.respondWith(c, response, info, http.StatusCreated)
	case http.StatusNoContent:
		c.Writer.WriteHeader(code)
		return nil
	case http.StatusAccepted:
		c.Writer.WriteHeader(code)
		return nil
	default:
		return fmt.Errorf("invalid status code %d from resource %s for method Create",
			code, res.name)
	}
}

func (res *resource) handleUpdate(c *gin.Context, info information) error {
	id := c.Param("id")
	obj, err := res.source.FindOne(id, buildReqParams(c))
	if err != nil {
		return err
	}
	rc := c.Request.Body
	// we have to make the Result to a pointer to unmarshal into it
	updatingObj := reflect.ValueOf(obj.Result())
	if updatingObj.Kind() == reflect.Struct {
		updatingObjPtr := reflect.New(reflect.TypeOf(obj.Result()))
		updatingObjPtr.Elem().Set(updatingObj)
		err = jsonapi.UnmarshalPayload(rc, updatingObjPtr.Interface())
		updatingObj = updatingObjPtr.Elem()
	} else {
		err = jsonapi.UnmarshalPayload(rc, updatingObj.Interface())
	}
	rc.Close()
	if err != nil {
		return NewHTTPError(nil, err.Error(), http.StatusNotAcceptable)
	}
	response, err := res.source.Update(updatingObj.Interface(),
		buildReqParams(c))
	if err != nil {
		return err
	}
	switch response.StatusCode() {
	case http.StatusOK:
		updated := response.Result()
		if updated == nil {
			internalResponse, err := res.source.FindOne(id, buildReqParams(c))
			if err != nil {
				return err
			}
			updated = internalResponse.Result()
			if updated == nil {
				return fmt.Errorf("Expected FindOne to return one object of resource %s",
					res.name)
			}
			response = internalResponse
		}
		return res.respondWith(c, response, info, http.StatusOK)
	case http.StatusAccepted:
		c.Writer.WriteHeader(http.StatusAccepted)
		return nil
	case http.StatusNoContent:
		c.Writer.WriteHeader(http.StatusNoContent)
		return nil
	default:
		return fmt.Errorf("invalid status code %d from resource %s for method Update",
			response.StatusCode(), res.name)
	}
}

func (res *resource) handleReplaceRelation(c *gin.Context,
	relation relationship) error {
	var (
		err     error
		editObj interface{}
	)
	id := c.Param(idStr)
	response, err := res.source.FindOne(id, buildReqParams(c))
	if err != nil {
		return err
	}
	body, err := unmarshalRequest(c.Request)
	if err != nil {
		return err
	}
	inc := map[string]interface{}{}
	err = json.Unmarshal(body, &inc)
	if err != nil {
		return err
	}
	data, ok := inc["data"]
	if !ok {
		return errors.New("Invalid object. Need a \"data\" object")
	}
	resType := reflect.TypeOf(response.Result()).Kind()
	if resType == reflect.Struct {
		editObj = getPointerToStruct(response.Result())
	} else {
		editObj = response.Result()
	}
	err = processRelationshipsData(data, relation.name, editObj)
	if err != nil {
		return err
	}
	if resType == reflect.Struct {
		_, err = res.source.Update(reflect.ValueOf(editObj).Elem().Interface(),
			buildReqParams(c))
	} else {
		_, err = res.source.Update(editObj, buildReqParams(c))
	}
	c.Writer.WriteHeader(http.StatusNoContent)
	return err
}

func (res *resource) handleAddToManyRelation(c *gin.Context,
	relation relationship) error {
	var (
		err     error
		editObj interface{}
	)
	id := c.Param(idStr)
	response, err := res.source.FindOne(id, buildReqParams(c))
	if err != nil {
		return err
	}
	body, err := unmarshalRequest(c.Request)
	if err != nil {
		return err
	}
	inc := map[string]interface{}{}
	err = json.Unmarshal(body, &inc)
	if err != nil {
		return err
	}
	data, ok := inc["data"]
	if !ok {
		return errors.New("Invalid object. Need a \"data\" object")
	}
	newRels, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("Data must be an array with \"id\" and \"type\" field to add new to-many relationships")
	}
	newIDs := []string{}
	for _, newRel := range newRels {
		casted, ok := newRel.(map[string]interface{})
		if !ok {
			return errors.New("entry in data object invalid")
		}
		newID, ok := casted["id"].(string)
		if !ok {
			return errors.New("no id field found inside data object")
		}
		newIDs = append(newIDs, newID)
	}
	resType := reflect.TypeOf(response.Result()).Kind()
	if resType == reflect.Struct {
		editObj = getPointerToStruct(response.Result())
	} else {
		editObj = response.Result()
	}
	targetObj, ok := editObj.(EditToManyRelations)
	if !ok {
		return errors.New("target struct must implement jsonapi.EditToManyRelations")
	}
	targetObj.AddToManyIDs(relation.name, newIDs)
	if resType == reflect.Struct {
		_, err = res.source.Update(reflect.ValueOf(targetObj).Elem().Interface(),
			buildReqParams(c))
	} else {
		_, err = res.source.Update(targetObj, buildReqParams(c))
	}
	c.Writer.WriteHeader(http.StatusNoContent)
	return err
}

func (res *resource) handleDeleteToManyRelation(c *gin.Context,
	relation relationship) error {
	var (
		err     error
		editObj interface{}
	)
	id := c.Param(idStr)
	response, err := res.source.FindOne(id, buildReqParams(c))
	if err != nil {
		return err
	}
	body, err := unmarshalRequest(c.Request)
	if err != nil {
		return err
	}
	inc := map[string]interface{}{}
	err = json.Unmarshal(body, &inc)
	if err != nil {
		return err
	}
	data, ok := inc["data"]
	if !ok {
		return errors.New("Invalid object. Need a \"data\" object")
	}
	newRels, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("Data must be an array with \"id\" and \"type\" field to add new to-many relationships")
	}
	obsoleteIDs := []string{}
	for _, newRel := range newRels {
		casted, ok := newRel.(map[string]interface{})
		if !ok {
			return errors.New("entry in data object invalid")
		}
		obsoleteID, ok := casted["id"].(string)
		if !ok {
			return errors.New("no id field found inside data object")
		}
		obsoleteIDs = append(obsoleteIDs, obsoleteID)
	}
	resType := reflect.TypeOf(response.Result()).Kind()
	if resType == reflect.Struct {
		editObj = getPointerToStruct(response.Result())
	} else {
		editObj = response.Result()
	}
	targetObj, ok := editObj.(EditToManyRelations)
	if !ok {
		return errors.New("target struct must implement jsonapi.EditToManyRelations")
	}
	targetObj.DeleteToManyIDs(relation.name, obsoleteIDs)
	if resType == reflect.Struct {
		_, err = res.source.Update(reflect.ValueOf(targetObj).Elem().Interface(),
			buildReqParams(c))
	} else {
		_, err = res.source.Update(targetObj, buildReqParams(c))
	}
	c.Writer.WriteHeader(http.StatusNoContent)
	return err
}

// returns a pointer to an interface{} struct
func getPointerToStruct(oldObj interface{}) interface{} {
	resType := reflect.TypeOf(oldObj)
	ptr := reflect.New(resType)
	ptr.Elem().Set(reflect.ValueOf(oldObj))
	return ptr.Interface()
}

func (res *resource) handleDelete(c *gin.Context) error {
	id := c.Param(idStr)
	response, err := res.source.Delete(id, buildReqParams(c))
	if err != nil {
		return err
	}
	w := c.Writer
	switch response.StatusCode() {
	case http.StatusOK:
		var m *jsonapi.Meta
		if metable, ok := response.(Metable); ok {
			m = metable.Metadata()
		}
		data := map[string]interface{}{
			"meta": *m,
		}
		return res.marshalResponse(c, data, http.StatusOK)
	case http.StatusAccepted:
		w.WriteHeader(http.StatusAccepted)
		return nil
	case http.StatusNoContent:
		w.WriteHeader(http.StatusNoContent)
		return nil
	default:
		return fmt.Errorf("invalid status code %d from resource %s for method Delete",
			response.StatusCode(), res.name)
	}
}

func writeResult(w http.ResponseWriter, data []byte, status int, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	w.Write(data)
}

func (res *resource) respondWith(c *gin.Context, obj Responder,
	info information, status int) error {
	doc, err := marshalToDoc(obj.Result(), info)
	if err != nil {
		return err
	}
	if metable, ok := obj.(Metable); ok {
		meta := metable.Metadata()
		if len(*meta) > 0 {
			doc.meta(meta)
		}
	}
	if objWithLinks, ok := obj.(LinksResponder); ok {
		links := objWithLinks.Links(c.Request, info)
		if len(*links) > 0 {
			doc.links(links)
		}
	}
	return res.marshalResponse(c, doc, status)
}

func (res *resource) respondWithPagination(c *gin.Context, obj Responder,
	info information, status int, links *jsonapi.Links) error {
	doc, err := marshalToDoc(obj.Result(), info)
	if err != nil {
		return err
	}
	doc.links(links)
	if metable, ok := obj.(Metable); ok {
		meta := metable.Metadata()
		if len(*meta) > 0 {
			doc.meta(meta)
		}
	}
	return res.marshalResponse(c, doc, status)
}

func unmarshalRequest(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func filterSparseFields(resp interface{}, c *gin.Context) (interface{}, error) {
	query := c.Request.URL.Query()
	queryParams := parseQueryFields(&query)
	if len(queryParams) < 1 {
		return resp, nil
	}
	if document, ok := resp.(*Doc); ok {
		wrongFields := map[string][]string{}

		// single entry in data
		one := document.node()
		if one != nil {
			errors := replaceAttributes(&queryParams, one)
			for t, v := range errors {
				wrongFields[t] = v
			}
		}

		many := document.nodes()
		if many != nil {
			for _, data := range many {
				errors := replaceAttributes(&queryParams, data)
				for t, v := range errors {
					wrongFields[t] = v
				}
			}
		}

		// included slice
		for _, include := range document.included() {
			errors := replaceAttributes(&queryParams, include)
			for t, v := range errors {
				wrongFields[t] = v
			}
		}

		if len(wrongFields) > 0 {
			httpError := NewHTTPError(nil, "Some requested fields were invalid",
				http.StatusBadRequest)
			for k, v := range wrongFields {
				for _, field := range v {
					httpError.E = append(httpError.E, &jsonapi.ErrorObject{
						Status: http.StatusText(http.StatusBadRequest),
						Code:   codeInvalidQueryFields,
						Title: fmt.Sprintf(`Field "%s" does not exist for type "%s"`,
							field, k),
						Detail: "Please make sure you do only request existing fields",
					})
				}
			}
			return nil, httpError
		}
	}
	return resp, nil
}

func parseQueryFields(query *url.Values) (result map[string][]string) {
	result = map[string][]string{}
	for name, param := range *query {
		matches := queryFieldsRegex.FindStringSubmatch(name)
		if len(matches) > 1 {
			match := matches[1]
			result[match] = strings.Split(param[0], ",")
		}
	}
	return
}

func filterAttributes(attributes map[string]interface{},
	fields []string) (filteredAttributes map[string]interface{},
	wrongFields []string) {
	wrongFields = []string{}
	filteredAttributes = map[string]interface{}{}

	for _, field := range fields {
		if attribute, ok := attributes[field]; ok {
			filteredAttributes[field] = attribute
		} else {
			wrongFields = append(wrongFields, field)
		}
	}
	return
}

func replaceAttributes(query *map[string][]string,
	node *jsonapi.Node) map[string][]string {
	fieldType := node.Type
	fields := (*query)[fieldType]
	if len(fields) > 0 && len(node.Attributes) > 0 {
		attributes, wrongFields := filterAttributes(node.Attributes, fields)
		if len(wrongFields) > 0 {
			return map[string][]string{
				fieldType: wrongFields,
			}
		}
		node.Attributes = attributes
	}
	return nil
}

func (api *API) handleError(err error, c *gin.Context) {
	log.Println(err)
	if e, ok := err.(HTTPError); ok {
		c.Render(e.status, e)
		return
	}
	c.Render(http.StatusInternalServerError, NewHTTPError(err, err.Error(),
		http.StatusInternalServerError))
}

// TODO: this can also be replaced with a struct into that we directly json.Unmarshal
func processRelationshipsData(data interface{}, linkName string,
	target interface{}) error {
	hasOne, ok := data.(map[string]interface{})
	if ok {
		hasOneID, ok := hasOne["id"].(string)
		if !ok {
			return fmt.Errorf("data object must have a field id for %s",
				linkName)
		}
		target, ok := target.(UnmarshalToOneRelations)
		if !ok {
			return errors.New("target struct must implement interface UnmarshalToOneRelations")
		}
		target.SetToOneReferenceID(linkName, hasOneID)
	} else if data == nil {
		// this means that a to-one relationship must be deleted
		target, ok := target.(UnmarshalToOneRelations)
		if !ok {
			return errors.New("target struct must implement interface UnmarshalToOneRelations")
		}
		target.SetToOneReferenceID(linkName, "")
	} else {
		hasMany, ok := data.([]interface{})
		if !ok {
			return fmt.Errorf("invalid data object or array, must be an object with \"id\" and \"type\" field for %s",
				linkName)
		}
		target, ok := target.(UnmarshalToManyRelations)
		if !ok {
			return errors.New("target struct must implement interface UnmarshalToManyRelations")
		}
		hasManyIDs := []string{}
		for _, entry := range hasMany {
			data, ok := entry.(map[string]interface{})
			if !ok {
				return fmt.Errorf("entry in data array must be an object for %s",
					linkName)
			}
			dataID, ok := data["id"].(string)
			if !ok {
				return fmt.Errorf("all data objects must have a field id for %s",
					linkName)
			}
			hasManyIDs = append(hasManyIDs, dataID)
		}
		target.SetToManyReferenceIDs(linkName, hasManyIDs)
	}
	return nil
}
