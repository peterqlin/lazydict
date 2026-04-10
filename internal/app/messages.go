package app

import "github.com/230pe/lazydict/internal/api"

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
