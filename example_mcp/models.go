package api

type Note struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type AddNoteArgs struct {
	Text string `json:"text"`
}

type AddNoteResult struct {
	ID string `json:"id"`
}
