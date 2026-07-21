package models

const (
	IsicCodePerPage = 10
	IsicCodePath    = "/api/isic-codes"
)

// IsicCode is an ISIC classification code.
type IsicCode struct {
	ID       uint64
	Name     string
	Code     *uint64
	Verified bool
}

// IsicCodePage is a paginated list of ISIC codes.
type IsicCodePage struct {
	Items       []IsicCode
	CurrentPage int
	PerPage     int
	Total       int
	LastPage    int
	From        *int
	To          *int
	Path        string
	Search      string
}
