package api2go

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/cention-sany/jsonapi"
)

type noder interface {
	node() *jsonapi.Node
	nodes() []*jsonapi.Node
	meta(v ...*jsonapi.Meta) *jsonapi.Meta
	links(v ...*jsonapi.Links) *jsonapi.Links
}

// Doc implements noder and MarshalJSON
type Doc struct {
	one  *jsonapi.OnePayload
	many *jsonapi.ManyPayload
}

func (d *Doc) node() *jsonapi.Node {
	return d.one.Data
}

func (d *Doc) nodes() []*jsonapi.Node {
	return d.many.Data
}

func (d *Doc) included() []*jsonapi.Node {
	if d.one != nil {
		return d.one.Included
	} else if d.many != nil {
		return d.many.Included
	}
	return nil
}

func (d *Doc) meta(v ...*jsonapi.Meta) *jsonapi.Meta {
	var (
		m     *jsonapi.Meta
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

func (d *Doc) links(v ...*jsonapi.Links) *jsonapi.Links {
	var (
		l     *jsonapi.Links
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
	one  *jsonapi.RelationshipOneNode
	many *jsonapi.RelationshipManyNode
}

func (r *RelationNode) node() *jsonapi.Node {
	return r.one.Data
}

func (r *RelationNode) nodes() []*jsonapi.Node {
	return r.many.Data
}

func (r *RelationNode) meta(v ...*jsonapi.Meta) *jsonapi.Meta {
	var (
		m     *jsonapi.Meta
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

func (r *RelationNode) links(v ...*jsonapi.Links) *jsonapi.Links {
	var (
		l     *jsonapi.Links
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
		return nil, errors.New("api2go: nothing to marshal")
	}
	switch reflect.TypeOf(v).Kind() {
	case reflect.Slice:
		value := reflect.ValueOf(v)
		size := value.Len()
		vs := make([]interface{}, 0, size)
		for i := 0; i < size; i++ {
			vs = append(vs, value.Index(i).Interface())
		}
		many, err := jsonapi.MarshalManyWithSI(vs, info)
		if err != nil {
			return nil, err
		}
		return &Doc{many: many}, nil
	case reflect.Struct, reflect.Ptr:
		one, err := jsonapi.MarshalOneWithSI(v, info)
		if err != nil {
			return nil, err
		}
		return &Doc{one: one}, nil
	default:
		return nil, errors.New("Marshal only accepts slice, struct or ptr types")
	}
}

type DefaultLinks struct {
	id, name     string
	withRelation bool
}

func NewDefaultLinks(id, name string, withRelation bool) *DefaultLinks {
	return &DefaultLinks{id: id, name: name, withRelation: withRelation}
}

func (d DefaultLinks) LinksWithSI(si jsonapi.ServerInformation) *jsonapi.Links {
	result := make(jsonapi.Links)
	s := fmt.Sprint(si.GetBaseURL(), si.GetPrefix(), "/", d.name)
	if d.id != "" {
		s = fmt.Sprint(s, "/", d.id)
	}
	result["self"] = jsonapi.Link{Href: s}
	return &result
}

const relStr = "relationships/"

func (d DefaultLinks) RelationshipLinksWithSI(r string,
	si jsonapi.ServerInformation) *jsonapi.Links {
	if !d.withRelation {
		return nil
	}
	result := make(jsonapi.Links)
	result["self"] = jsonapi.Link{Href: fmt.Sprint(si.GetBaseURL(),
		si.GetPrefix(), "/", d.name, "/", d.id, relStr, r)}
	result["related"] = jsonapi.Link{Href: fmt.Sprint(si.GetBaseURL(),
		si.GetPrefix(), "/", d.name, "/", d.id, "/", r)}
	return &result
}
