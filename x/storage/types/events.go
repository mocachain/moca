package types

// Event type and attribute keys for events the module emits outside the generated
// proto event set.
const (
	// EventTypeDiscontinueDeleteFailed is emitted when a discontinued bucket or object could
	// not be deleted by the end-block garbage collector. Nothing is written for the resource
	// and its id is dropped from the deletion queue, so it stays discontinued and uncollected.
	EventTypeDiscontinueDeleteFailed = "discontinue_delete_failed"

	// AttributeKeyResourceType is the type of the resource that failed to be deleted.
	AttributeKeyResourceType = "resource_type"
	// AttributeKeyResourceID is the u256 id of the bucket or object that failed to be deleted.
	AttributeKeyResourceID = "resource_id"
	// AttributeKeyError describes why the deletion failed.
	AttributeKeyError = "error"
)
