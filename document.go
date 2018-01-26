package api2go

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"

	ja "github.com/cention-sany/jsonapi"
)

var (
	// EmptyObject only apply to top level object which return null when the doc
	// can not be found but the requested path is legitimate.
	EmptyObject = &ja.OnePayload{}
	// EmptyObject only apply to top level object which return empty json array
	// [] when the doc can not be found but the requested path is legitimate.
	EmptyArray = &ja.ManyPayload{}
)

type noder interface {
	node() *ja.Node
	nodes() []*ja.Node
	meta(v ...*ja.Meta) *ja.Meta
	links(v ...*ja.Links) *ja.Links
}

// Doc implements noder and MarshalJSON
type Doc struct {
	one  *ja.OnePayload
	many *ja.ManyPayload
}

func (d *Doc) node() *ja.Node {
	if d.one == nil {
		return nil
	}
	return d.one.Data
}

func (d *Doc) nodes() []*ja.Node {
	if d.many == nil {
		return nil
	}
	return d.many.Data
}

func (d *Doc) included() []*ja.Node {
	if d.one != nil {
		return d.one.Included
	} else if d.many != nil {
		return d.many.Included
	}
	return nil
}

func (d *Doc) meta(v ...*ja.Meta) *ja.Meta {
	var (
		m     *ja.Meta
		isSet bool
	)
	if v != nil && len(v) > 0 {
		m = v[0]
		isSet = true
	}
	if isSet {
		if d.one != nil {
			d.one.Meta = m
		} else if d.many != nil {
			d.many.Meta = m
		}
		return m
	}
	if d.one != nil {
		return d.one.Meta
	} else if d.many != nil {
		return d.many.Meta
	}
	return nil
}

func (d *Doc) links(v ...*ja.Links) *ja.Links {
	var (
		l     *ja.Links
		isSet bool
	)
	if v != nil && len(v) > 0 {
		l = v[0]
		isSet = true
	}
	if isSet {
		if d.one != nil {
			d.one.Links = l
		} else if d.many != nil {
			d.many.Links = l
		}
		return l
	}
	if d.one != nil {
		return d.one.Links
	} else if d.many != nil {
		return d.many.Links
	}
	return nil
}

func (d *Doc) MarshalJSON() ([]byte, error) {
	if d.one != nil {
		return json.Marshal(d.one)
	} else if d.many != nil {
		return json.Marshal(d.many)
	}
	return nil, errors.New("api2go: no document to marshal")
}

// RelationNode implements noder and MarshalJSON
type RelationNode struct {
	one  *ja.RelationshipOneNode
	many *ja.RelationshipManyNode
}

func (r *RelationNode) node() *ja.Node {
	return r.one.Data
}

func (r *RelationNode) nodes() []*ja.Node {
	return r.many.Data
}

func (r *RelationNode) meta(v ...*ja.Meta) *ja.Meta {
	var (
		m     *ja.Meta
		isSet bool
	)
	if v != nil && len(v) > 0 {
		m = v[0]
		isSet = true
	}
	if isSet {
		if r.one != nil {
			r.one.Meta = m
		} else if r.many != nil {
			r.many.Meta = m
		}
		return m
	}
	if r.one != nil {
		return r.one.Meta
	} else if r.many != nil {
		return r.many.Meta
	}
	return nil
}

func (r *RelationNode) links(v ...*ja.Links) *ja.Links {
	var (
		l     *ja.Links
		isSet bool
	)
	if v != nil && len(v) > 0 {
		l = v[0]
		isSet = true
	}
	if isSet {
		if r.one != nil {
			r.one.Links = l
		} else if r.many != nil {
			r.many.Links = l
		}
		return l
	}
	if r.one != nil {
		return r.one.Links
	} else if r.many != nil {
		return r.many.Links
	}
	return nil
}

func (r *RelationNode) MarshalJSON() ([]byte, error) {
	if r.one != nil {
		return json.Marshal(r.one)
	} else if r.many != nil {
		return json.Marshal(r.many)
	}
	return nil, errors.New("api2go: no relation node to marshal")
}

func marshalToDoc(v interface{}, info information) (*Doc, error) {
	if v == nil {
		return &Doc{one: EmptyObject}, nil
	}
	value := reflect.ValueOf(v)
	k := value.Kind()
	switch k {
	case reflect.Slice:
		size := value.Len()
		if size == 0 {
			return &Doc{many: EmptyArray}, nil
		}
		vs := make([]interface{}, 0, size)
		for i := 0; i < size; i++ {
			vs = append(vs, value.Index(i).Interface())
		}
		many, err := ja.MarshalManyWithSI(vs, info)
		if err != nil {
			return nil, err
		}
		return &Doc{many: many}, nil
	case reflect.Struct, reflect.Ptr:
		if k == reflect.Struct {
			if reflect.Zero(value.Type()).Interface() == v {
				return &Doc{one: EmptyObject}, nil
			}
		} else if value.IsNil() {
			return &Doc{one: EmptyObject}, nil
		}
		one, err := ja.MarshalOneWithSI(v, info)
		if err != nil {
			return nil, err
		}
		return &Doc{one: one}, nil
	default:
		return nil, errors.New("Marshal only accepts slice, struct or ptr types")
	}
}

// DefaultLinks is helper struct to generate link object for Responder or any
// struct that embeds it. DefaultLinks handle nil value by return nil to
// LinksWithSI and RelationshipLinksWithSI to avoid any links object to be
// generated. Thus provide flexibility to embedding struct to control the link
// object generations.
type DefaultLinks struct {
	id, name     string
	withRelation bool
}

func NewDefaultLinks(id, name string, withRelation bool) *DefaultLinks {
	return &DefaultLinks{id: id, name: name, withRelation: withRelation}
}

func (d *DefaultLinks) LinksWithSI(si ja.ServerInformation) *ja.Links {
	if d == nil {
		return nil
	}
	result := make(ja.Links)
	s := fmt.Sprint(si.GetBaseURL(), si.GetPrefix(), d.name)
	if d.id != "" {
		s = fmt.Sprint(s, "/", d.id)
	}
	result["self"] = ja.Link{Href: s}
	return &result
}

const relStr = "relationships/"

func (d *DefaultLinks) RelationshipLinksWithSI(r string,
	si ja.ServerInformation) *ja.Links {
	if d == nil || !d.withRelation {
		return nil
	}
	result := make(ja.Links)
	result["self"] = ja.Link{Href: fmt.Sprint(si.GetBaseURL(),
		si.GetPrefix(), d.name, "/", d.id, "/", relStr, r)}
	result["related"] = ja.Link{Href: fmt.Sprint(si.GetBaseURL(),
		si.GetPrefix(), d.name, "/", d.id, "/", r)}
	return &result
}

// OffsetPage is helper to check if r contains pagination query or not and turn
// it into SQL friendly offset and limit value. It return if r contains page
// query, offset, limit, and any error found. If page[limit] not exist in the
// offset-paged query, limit will return as -1.
func OffsetPage(r *Request) (bool, int, int, error) {
	var (
		numOrOffI, sizeOrLmtI int
		err                   error
	)
	if len(r.Pagination) <= 0 {
		return false, 0, 0, nil
	}
	numOrOff, hasNumOrOff := r.Pagination["number"]
	sizeOrLmt, hasSizeOrLmt := r.Pagination["size"]
	if hasSizeOrLmt {
		sizeOrLmtI, err = strconv.Atoi(sizeOrLmt)
		if err != nil {
			return false, 0, 0, err
		} else if sizeOrLmtI <= 0 {
			return false, 0, 0, errors.New("api2go: invalid page size")
		}
		if hasNumOrOff {
			numOrOffI, err = strconv.Atoi(numOrOff)
			if err != nil {
				return false, 0, 0, err
			} else if numOrOffI <= 0 {
				return false, 0, 0, errors.New("api2go: invalid page number")
			}
		}
		// turn to offset
		if numOrOffI > 0 {
			// query contains page number
			return true, sizeOrLmtI * (numOrOffI - 1), sizeOrLmtI, nil
		}
		return true, 0, sizeOrLmtI, nil
	}
	numOrOff, hasNumOrOff = r.Pagination["offset"]
	sizeOrLmt, hasSizeOrLmt = r.Pagination["limit"]
	if !hasNumOrOff && !hasSizeOrLmt {
		return false, 0, 0, nil
	}
	if hasSizeOrLmt {
		sizeOrLmtI, err = strconv.Atoi(sizeOrLmt)
		if err != nil {
			return false, 0, 0, err
		} else if sizeOrLmtI <= 0 {
			return false, 0, 0, errors.New("api2go: invalid page limit")
		}
	} else {
		sizeOrLmtI = -1
	}
	if hasNumOrOff {
		numOrOffI, err = strconv.Atoi(numOrOff)
		if err != nil {
			return false, 0, 0, err
		}
	}
	return true, numOrOffI, sizeOrLmtI, nil
}
