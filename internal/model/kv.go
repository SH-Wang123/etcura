package model

// KeySummary is one row in the key list. It deliberately carries no value:
// values can be megabytes and are only fetched on demand (see GetKey).
type KeySummary struct {
	Key            string `json:"key"`
	CreateRevision int64  `json:"createRevision"`
	ModRevision    int64  `json:"modRevision"`
	Version        int64  `json:"version"`
	Lease          int64  `json:"lease"`
}

// KeyPage is one server-side page of keys, addressed by an opaque cursor.
type KeyPage struct {
	Prefix     string       `json:"prefix"`
	Cursor     string       `json:"cursor"`     // range start used for this page
	NextCursor string       `json:"nextCursor"` // range start for the following page
	HasMore    bool         `json:"hasMore"`
	Revision   int64        `json:"revision"` // etcd header revision of this read
	Items      []KeySummary `json:"items"`
}

// KvDetail is the full record of a single key including its value. The value
// is always transported as base64 so binary content survives the JSON bridge
// byte-exact.
type KvDetail struct {
	Key            string `json:"key"`
	ValueBase64    string `json:"valueBase64"`
	IsUTF8         bool   `json:"isUTF8"`
	SizeBytes      int    `json:"sizeBytes"`
	CreateRevision int64  `json:"createRevision"`
	ModRevision    int64  `json:"modRevision"`
	Version        int64  `json:"version"`
	Lease          int64  `json:"lease"`
}

// KeySegment is one next-level "directory" of the key tree, e.g.
// name "benchmark/" with the full prefix "/etcd-eye-test/benchmark/".
type KeySegment struct {
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
}

// SegmentPage is the result of a tree-level discovery under one prefix.
type SegmentPage struct {
	Prefix    string       `json:"prefix"`
	Segments  []KeySegment `json:"segments"`
	Truncated bool         `json:"truncated"` // more segments exist beyond the cap
}

// StatusResponse is returned by TestConnection.
type StatusResponse struct {
	Ok          bool   `json:"ok"`
	Header      int64  `json:"header"`
	EtcdVersion string `json:"etcdVersion"`
	Endpoint    string `json:"endpoint"`
}
