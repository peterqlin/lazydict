package app

import "github.com/peterqlin/lazydict/internal/api"

type WordFetchedMsg struct {
	Word  string
	Entry *api.Entry
}

type FetchErrMsg struct {
	Word string
	Err  error
}

type NotFoundMsg struct {
	Word        string
	Suggestions []string
}
