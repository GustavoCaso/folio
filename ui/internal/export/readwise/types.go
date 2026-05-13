package readwise

type ReadwiseHighlightInput struct {
	Text          string `json:"text"`
	Title         string `json:"title"`
	Category      string `json:"category"`
	Note          string `json:"note,omitempty"`
	HighlightedAt string `json:"highlighted_at,omitempty"`
}

type ReadwiseCreateRequest struct {
	Highlights []ReadwiseHighlightInput `json:"highlights"`
}

// Readwise POST /api/v2/highlights/ returns a plain JSON array, not a wrapped object.
type ReadwiseHighlightResponse struct {
	ModifiedHighlights []int64 `json:"modified_highlights"`
}

// Readwise POST /api/v2/highlights/<highlight_id>/tags/"
type ReadwiseTagRequest struct {
	Name string `json:"name"`
}
