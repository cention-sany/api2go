package api2go

// The Identifier interface is necessary to give an element a unique ID.
//
// Note: The implementation of this interface is mandatory.
type Identifier interface {
	GetID() string
}

// The UnmarshalIdentifier interface must be implemented to set the ID during
// unmarshalling.
type UnmarshalIdentifier interface {
	SetID(string) error
}

// The UnmarshalToOneRelations interface must be implemented to unmarshal
// to-one relations.
type UnmarshalToOneRelations interface {
	SetToOneReferenceID(name, ID string) error
}

// The UnmarshalToManyRelations interface must be implemented to unmarshal
// to-many relations.
type UnmarshalToManyRelations interface {
	SetToManyReferenceIDs(name string, IDs []string) error
}

// The EditToManyRelations interface can be optionally implemented to add and
// delete to-many relationships on a already unmarshalled struct. These methods
// are used by our API for the to-many relationship update routes.
//
// There are 3 HTTP Methods to edit to-many relations:
//
//	PATCH /v1/posts/1/comments
//	Content-Type: application/vnd.api+json
//	Accept: application/vnd.api+json
//
//	{
//	  "data": [
//		{ "type": "comments", "id": "2" },
//		{ "type": "comments", "id": "3" }
//	  ]
//	}
//
// This replaces all of the comments that belong to post with ID 1 and the
// SetToManyReferenceIDs method will be called.
//
//	POST /v1/posts/1/comments
//	Content-Type: application/vnd.api+json
//	Accept: application/vnd.api+json
//
//	{
//	  "data": [
//		{ "type": "comments", "id": "123" }
//	  ]
//	}
//
// Adds a new comment to the post with ID 1.
// The AddToManyIDs method will be called.
//
//	DELETE /v1/posts/1/comments
//	Content-Type: application/vnd.api+json
//	Accept: application/vnd.api+json
//
//	{
//	  "data": [
//		{ "type": "comments", "id": "12" },
//		{ "type": "comments", "id": "13" }
//	  ]
//	}
//
// Deletes comments that belong to post with ID 1.
// The DeleteToManyIDs method will be called.
type EditToManyRelations interface {
	AddToManyIDs(name string, IDs []string) error
	DeleteToManyIDs(name string, IDs []string) error
}
