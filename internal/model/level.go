package model

// LevelRow is one entry of a dynamic column in the multi-column browser:
// either a child directory (segment) or a direct leaf key of that level.
type LevelRow struct {
	Type        string `json:"type"` // "segment" | "leaf"
	Name        string `json:"name"` // relative display name ("benchmark/" or "c")
	Path        string `json:"path"` // full prefix (segment) or full key (leaf)
	ModRevision int64  `json:"modRevision"`
	Version     int64  `json:"version"`
	Lease       int64  `json:"lease"`
}

// LevelPage is one server-side page of a column's rows, addressed by an
// opaque cursor (same contract family as KeyPage).
type LevelPage struct {
	Prefix     string     `json:"prefix"`
	Cursor     string     `json:"cursor"`
	NextCursor string     `json:"nextCursor"`
	HasMore    bool       `json:"hasMore"`
	Rows       []LevelRow `json:"rows"`
}
