package api2go

import (
	"errors"
	"reflect"
	"strings"

	"github.com/cention-sany/jsonapi"
)

const (
	annotationJSONAPI   = "jsonapi"
	annotationSeperator = ","
	annotationPrimary   = "primary"
	annotationRelation  = "relation"
	defRelSize          = 4
)

type relationship struct {
	typ, name string
	isMany    bool
}

type nodeRelations struct {
	typ       string
	relations []*relationship
}

func findRelations(t reflect.Type, f map[string]bool) (*nodeRelations, error) {
	var (
		er   error
		node nodeRelations
	)
	k := t.Kind()
	if k == reflect.Ptr || k == reflect.Slice {
		t = t.Elem()
	}
	node.relations = make([]*relationship, 0, defRelSize)
	for i := 0; i < t.NumField(); i++ {
		structField := t.Field(i)
		tag := structField.Tag.Get(annotationJSONAPI)
		if tag == "" {
			continue
		}
		args := strings.Split(tag, annotationSeperator)
		if len(args) < 1 {
			er = jsonapi.ErrBadJSONAPIStructTag
			break
		}
		annotation := args[0]
		if annotation == annotationPrimary {
			if f[args[1]] {
				break
			}
			node.typ = args[1]
			f[node.typ] = true
		} else if annotation == annotationRelation {
			tt := structField.Type
			rel := &relationship{
				name:   args[1],
				isMany: tt.Kind() == reflect.Slice,
			}
			if rel.isMany {
				tt = tt.Elem()
			}
			// purpose ignore error here as without it can still work
			relationshipType, err := findPrimary(tt)
			if err != nil {
				return nil, err
			}
			rel.typ = relationshipType
			node.relations = append(node.relations, rel)
		}
	}
	if er != nil {
		return nil, er
	} else if node.typ == "" {
		return nil, errors.New("api2go: need primary")
	}
	return &node, nil
}

func findPrimary(t reflect.Type) (string, error) {
	var er error
	k := t.Kind()
	if k == reflect.Ptr || k == reflect.Slice {
		t = t.Elem()
	}
	for i := 0; i < t.NumField(); i++ {
		structField := t.Field(i)
		tag := structField.Tag.Get(annotationJSONAPI)
		if tag == "" {
			continue
		}
		args := strings.Split(tag, annotationSeperator)
		if len(args) < 1 {
			er = jsonapi.ErrBadJSONAPIStructTag
			break
		}
		annotation := args[0]
		if annotation == annotationPrimary {
			return args[1], nil
		}
	}
	if er != nil {
		return "", er
	}
	return "", errors.New("api2:go: can not find primary")
}
