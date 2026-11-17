package model

type CompareRequest struct {
	SourceSession string `json:"sourceSession"`
	TargetSession string `json:"targetSession"`
	SourcePrefix  string `json:"sourcePrefix"`
	TargetPrefix  string `json:"targetPrefix"`
}

type CompareInfo struct {
	ID             string `json:"id"`
	SourceRevision string `json:"sourceRevision"`
	TargetRevision string `json:"targetRevision"`
	TargetReadOnly bool   `json:"targetReadOnly"`
}

type CompareRow struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	State       string `json:"state"`
	SourceLease bool   `json:"sourceLease"`
}

type ComparePage struct {
	Rows       []CompareRow `json:"rows"`
	NextCursor string       `json:"nextCursor"`
	Scanned    int          `json:"scanned"`
}

type CompareDetail struct {
	Row    CompareRow   `json:"row"`
	Source *RecordValue `json:"source"`
	Target *RecordValue `json:"target"`
}

type RecordValue struct {
	Base64    string `json:"base64"`
	UTF8      bool   `json:"utf8"`
	Bytes     int    `json:"bytes"`
	Truncated bool   `json:"truncated"`
}

type SyncPlan struct {
	ID   string       `json:"id"`
	Rows []CompareRow `json:"rows"`
}

type SyncResult struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	State   string `json:"state"`
	Message string `json:"message"`
}

type WatchStart struct {
	Session string `json:"session"`
	Key     string `json:"key"`
	Prefix  bool   `json:"prefix"`
}

type WatchEvent struct {
	Type     string       `json:"type"`
	Key      string       `json:"key"`
	Revision string       `json:"revision"`
	SeenAt   int64        `json:"seenAt"`
	Before   *RecordValue `json:"before"`
	After    *RecordValue `json:"after"`
}

type WatchBatch struct {
	Events   []WatchEvent `json:"events"`
	State    string       `json:"state"`
	Revision string       `json:"revision"`
	Missed   int          `json:"missed"`
	Error    string       `json:"error"`
}
