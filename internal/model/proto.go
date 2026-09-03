package model

// Error code for proto import/decode failures.
const CodeProtoError = "PROTO_ERROR"

// ProtoFileInfo describes one imported .proto file.
type ProtoFileInfo struct {
	Name         string `json:"name"`
	Package      string `json:"package"`
	MessageCount int    `json:"messageCount"`
	EnumCount    int    `json:"enumCount"`
}

// ProtoCandidate is one message type that successfully decoded the value,
// ranked by a heuristic score (higher = more likely).
type ProtoCandidate struct {
	MessageName  string `json:"messageName"`
	Score        int    `json:"score"`
	SetFieldNums []int  `json:"setFieldNums"`
	UnknownBytes int    `json:"unknownBytes"`
	JSON         string `json:"json"` // protojson output (proto field names); empty when too large
}

// ProtoMatchResult is the outcome of matching a binary value against all
// imported message types.
type ProtoMatchResult struct {
	WireTagNums   []int            `json:"wireTagNums"` // top-level field numbers seen in the blob
	Scanned       int              `json:"scanned"`     // candidate types attempted
	ValueTooLarge bool             `json:"valueTooLarge"`
	Candidates    []ProtoCandidate `json:"candidates"` // best first
}

// ProtoImportSummary is returned after importing files.
type ProtoImportSummary struct {
	Files    []ProtoFileInfo `json:"files"`
	Messages int             `json:"messages"`
	Enums    int             `json:"enums"`
}
