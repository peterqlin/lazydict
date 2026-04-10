package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

const (
	defaultDictBase = "https://www.dictionaryapi.com/api/v3/references/collegiate/json/"
	defaultThesBase = "https://www.dictionaryapi.com/api/v3/references/thesaurus/json/"
)

// Entry is the normalised result for a single lookup.
type Entry struct {
	Word            string
	Pronunciation   string
	FunctionalLabel string
	Definitions     []string
	Synonyms        []string
	Antonyms        []string
	Examples        []string
	Forms           []string
	Etymology       string
}

// NotFoundError is returned when the API returns suggestions instead of entries.
type NotFoundError struct {
	Word        string
	Suggestions []string
}

func (e *NotFoundError) Error() string {
	tips := e.Suggestions
	if len(tips) > 3 {
		tips = tips[:3]
	}
	return fmt.Sprintf("no results for %q (did you mean: %s?)", e.Word, strings.Join(tips, ", "))
}

// Client is the MW API client.
type Client struct {
	dictKey  string
	thesKey  string
	dictBase string
	thesBase string
	http     *http.Client
}

// Option configures a Client.
type Option func(*Client)

func WithDictBaseURL(u string) Option { return func(c *Client) { c.dictBase = u } }
func WithThesBaseURL(u string) Option { return func(c *Client) { c.thesBase = u } }

// NewClient creates a Client with the given API keys.
func NewClient(dictKey, thesKey string, opts ...Option) *Client {
	c := &Client{
		dictKey:  dictKey,
		thesKey:  thesKey,
		dictBase: defaultDictBase,
		thesBase: defaultThesBase,
		http:     &http.Client{},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Fetch looks up word concurrently in the Dictionary and Thesaurus APIs.
func (c *Client) Fetch(word string) (*Entry, error) {
	type dictResult struct {
		entries     []rawDictEntry
		suggestions []string
		err         error
	}
	type thesResult struct {
		entries []rawThesEntry
		err     error
	}

	var wg sync.WaitGroup
	dictCh := make(chan dictResult, 1)
	thesCh := make(chan thesResult, 1)

	wg.Add(2)
	go func() {
		defer wg.Done()
		entries, suggestions, err := c.fetchDict(word)
		dictCh <- dictResult{entries, suggestions, err}
	}()
	go func() {
		defer wg.Done()
		entries, err := c.fetchThes(word)
		thesCh <- thesResult{entries, err}
	}()
	wg.Wait()

	dr := <-dictCh
	tr := <-thesCh

	if dr.err != nil {
		return nil, fmt.Errorf("dictionary API: %w", dr.err)
	}
	if len(dr.suggestions) > 0 {
		return nil, &NotFoundError{Word: word, Suggestions: dr.suggestions}
	}
	if len(dr.entries) == 0 {
		return nil, &NotFoundError{Word: word}
	}

	return buildEntry(word, dr.entries, tr.entries), nil
}

// --- raw API types ---

type rawDictEntry struct {
	Meta struct {
		ID    string   `json:"id"`
		Stems []string `json:"stems"`
	} `json:"meta"`
	HWI struct {
		HW  string `json:"hw"`
		Prs []struct {
			MW string `json:"mw"`
		} `json:"prs"`
	} `json:"hwi"`
	FL  string `json:"fl"`
	Def []struct {
		SSeq json.RawMessage `json:"sseq"`
	} `json:"def"`
	ET  json.RawMessage `json:"et"`
	Ins []struct {
		IL string `json:"il"`
		IF string `json:"if"`
	} `json:"ins"`
}

type rawThesEntry struct {
	Meta struct {
		Syns [][]string `json:"syns"`
		Ants [][]string `json:"ants"`
	} `json:"meta"`
}

// --- HTTP helpers ---

func (c *Client) fetchDict(word string) ([]rawDictEntry, []string, error) {
	url := c.dictBase + word + "?key=" + c.dictKey
	body, err := c.get(url)
	if err != nil {
		return nil, nil, err
	}
	return parseDict(body)
}

func (c *Client) fetchThes(word string) ([]rawThesEntry, error) {
	url := c.thesBase + word + "?key=" + c.thesKey
	body, err := c.get(url)
	if err != nil {
		return nil, err
	}
	var entries []rawThesEntry
	_ = json.Unmarshal(body, &entries)
	return entries, nil
}

func (c *Client) get(url string) ([]byte, error) {
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// --- parsing ---

func parseDict(body []byte) ([]rawDictEntry, []string, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil || len(raw) == 0 {
		return nil, nil, nil
	}
	var firstStr string
	if err := json.Unmarshal(raw[0], &firstStr); err == nil {
		suggestions := make([]string, 0, len(raw))
		for _, r := range raw {
			var s string
			if json.Unmarshal(r, &s) == nil {
				suggestions = append(suggestions, s)
			}
		}
		return nil, suggestions, nil
	}
	var entries []rawDictEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, nil, err
	}
	return entries, nil, nil
}

func buildEntry(word string, dict []rawDictEntry, thes []rawThesEntry) *Entry {
	e := &Entry{Word: word}
	if len(dict) == 0 {
		return e
	}
	first := dict[0]
	if len(first.HWI.Prs) > 0 {
		e.Pronunciation = first.HWI.Prs[0].MW
	}

	seen := map[string]bool{first.FL: true}
	labels := []string{first.FL}
	for _, d := range dict[1:] {
		if !seen[d.FL] {
			seen[d.FL] = true
			labels = append(labels, d.FL)
		}
	}
	e.FunctionalLabel = strings.Join(labels, " / ")

	for _, d := range dict {
		for _, def := range d.Def {
			defs := extractDefs(def.SSeq)
			e.Definitions = append(e.Definitions, defs...)
		}
	}

	e.Etymology = extractEtymology(first.ET)

	formsSeen := map[string]bool{}
	for _, d := range dict {
		for _, ins := range d.Ins {
			f := CleanMarkup(ins.IF)
			if !formsSeen[f] {
				formsSeen[f] = true
				e.Forms = append(e.Forms, f)
			}
		}
	}

	if len(thes) > 0 {
		for _, group := range thes[0].Meta.Syns {
			e.Synonyms = append(e.Synonyms, group...)
		}
		for _, group := range thes[0].Meta.Ants {
			e.Antonyms = append(e.Antonyms, group...)
		}
	}

	return e
}

func extractDefs(sseq json.RawMessage) []string {
	if sseq == nil {
		return nil
	}
	var outer [][][]json.RawMessage
	if err := json.Unmarshal(sseq, &outer); err != nil {
		return nil
	}
	var defs []string
	for _, group := range outer {
		for _, sense := range group {
			if len(sense) < 2 {
				continue
			}
			var senseType string
			if json.Unmarshal(sense[0], &senseType) != nil || senseType != "sense" {
				continue
			}
			var sd struct {
				SN string              `json:"sn"`
				DT [][]json.RawMessage `json:"dt"`
			}
			if json.Unmarshal(sense[1], &sd) != nil {
				continue
			}
			for _, dt := range sd.DT {
				if len(dt) < 2 {
					continue
				}
				var dtType string
				if json.Unmarshal(dt[0], &dtType) != nil || dtType != "text" {
					continue
				}
				var text string
				if json.Unmarshal(dt[1], &text) != nil {
					continue
				}
				text = CleanMarkup(text)
				if text == "" {
					continue
				}
				if sd.SN != "" {
					defs = append(defs, sd.SN+". "+text)
				} else {
					defs = append(defs, text)
				}
			}
		}
	}
	return defs
}

func extractEtymology(et json.RawMessage) string {
	if et == nil {
		return ""
	}
	var pairs [][]json.RawMessage
	if json.Unmarshal(et, &pairs) != nil {
		return ""
	}
	var parts []string
	for _, pair := range pairs {
		if len(pair) < 2 {
			continue
		}
		var t string
		if json.Unmarshal(pair[0], &t) != nil || t != "text" {
			continue
		}
		var text string
		if json.Unmarshal(pair[1], &text) != nil {
			continue
		}
		parts = append(parts, CleanMarkup(text))
	}
	return strings.Join(parts, " ")
}

var (
	mwTagRe = regexp.MustCompile(`\{[^}]+\}`)
	mwLdquo = regexp.MustCompile(`\{ldquo\}`)
	mwRdquo = regexp.MustCompile(`\{rdquo\}`)
)

// CleanMarkup removes MW API markup tags. Exported for tests.
func CleanMarkup(s string) string {
	s = mwLdquo.ReplaceAllString(s, "\u201c")
	s = mwRdquo.ReplaceAllString(s, "\u201d")
	s = mwTagRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}
