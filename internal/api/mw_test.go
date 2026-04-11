package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/peterqlin/lazydict/internal/api"
)

const dictFixture = `[{
  "meta": {"id": "record:1", "stems": ["record", "records"]},
  "hwi": {"hw": "rec*ord", "prs": [{"mw": "ˈre-kərd"}]},
  "fl": "noun",
  "def": [{
    "sseq": [[["sense", {"sn": "1", "dt": [["text", "{bc}a thing constituting {it}evidence{/it}"]]}]]]
  }],
  "et": [["text", "Middle English {it}record{/it}"]],
  "ins": [{"il": "plural", "if": "rec*ords"}]
}]`

const thesFixture = `[{
  "meta": {"syns": [["account", "chronicle"]], "ants": [["hide"]]}
}]`

func TestFetch_HappyPath(t *testing.T) {
	dictSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(dictFixture))
	}))
	defer dictSrv.Close()

	thesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(thesFixture))
	}))
	defer thesSrv.Close()

	client := api.NewClient("dict-key", "thes-key",
		api.WithDictBaseURL(dictSrv.URL+"/"),
		api.WithThesBaseURL(thesSrv.URL+"/"),
	)

	entry, err := client.Fetch("record")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if entry.Word != "record" {
		t.Errorf("Word = %q, want %q", entry.Word, "record")
	}
	if len(entry.DefinitionGroups) == 0 {
		t.Fatal("expected at least one definition group")
	}
	if entry.DefinitionGroups[0].POS != "noun" {
		t.Errorf("POS = %q, want noun", entry.DefinitionGroups[0].POS)
	}
	if len(entry.DefinitionGroups[0].Defs) == 0 {
		t.Error("expected at least one definition in group")
	}
	if entry.DefinitionGroups[0].Defs[0] != "a thing constituting evidence" {
		t.Errorf("def = %q", entry.DefinitionGroups[0].Defs[0])
	}
	if entry.Etymology != "Middle English record" {
		t.Errorf("etymology = %q", entry.Etymology)
	}
	if len(entry.Synonyms) == 0 || entry.Synonyms[0] != "account" {
		t.Errorf("synonyms = %v", entry.Synonyms)
	}
	if len(entry.Forms) == 0 {
		t.Error("expected at least one form")
	}
}

func TestFetch_NotFound(t *testing.T) {
	dictSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["recorder", "record-breaking"]`))
	}))
	defer dictSrv.Close()

	thesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer thesSrv.Close()

	client := api.NewClient("k", "k",
		api.WithDictBaseURL(dictSrv.URL+"/"),
		api.WithThesBaseURL(thesSrv.URL+"/"),
	)

	_, err := client.Fetch("xyzzy")
	if err == nil {
		t.Fatal("expected error for not-found word")
	}
	nf, ok := err.(*api.NotFoundError)
	if !ok {
		t.Fatalf("expected *api.NotFoundError, got %T: %v", err, err)
	}
	if len(nf.Suggestions) == 0 {
		t.Error("expected suggestions in NotFoundError")
	}
}

func TestCleanMarkup(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"{bc}a thing", "a thing"},
		{"{it}evidence{/it}", "evidence"},
		{"{ldquo}hello{rdquo}", "\u201chello\u201d"},
		{"plain text", "plain text"},
	}
	for _, c := range cases {
		got := api.CleanMarkup(c.in)
		if got != c.want {
			t.Errorf("CleanMarkup(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

const dictMultiPOSFixture = `[
  {
    "meta": {"id": "record:1", "stems": ["record"]},
    "hwi": {"hw": "record", "prs": [{"mw": "ˈre-kərd"}]},
    "fl": "noun",
    "def": [{"sseq": [[["sense", {"sn": "1", "dt": [["text", "a piece of evidence"]]}]]]}]
  },
  {
    "meta": {"id": "record:2", "stems": ["record"]},
    "hwi": {"hw": "record"},
    "fl": "verb",
    "def": [{"sseq": [[["sense", {"sn": "1", "dt": [["text", "to set down in writing"]]}]]]}]
  }
]`

func TestFetch_MultiPOS(t *testing.T) {
	dictSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(dictMultiPOSFixture))
	}))
	defer dictSrv.Close()

	thesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer thesSrv.Close()

	client := api.NewClient("k", "k",
		api.WithDictBaseURL(dictSrv.URL+"/"),
		api.WithThesBaseURL(thesSrv.URL+"/"),
	)

	entry, err := client.Fetch("record")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(entry.DefinitionGroups) != 2 {
		t.Fatalf("expected 2 POS groups, got %d", len(entry.DefinitionGroups))
	}
	if entry.DefinitionGroups[0].POS != "noun" {
		t.Errorf("group[0].POS = %q, want noun", entry.DefinitionGroups[0].POS)
	}
	if entry.DefinitionGroups[1].POS != "verb" {
		t.Errorf("group[1].POS = %q, want verb", entry.DefinitionGroups[1].POS)
	}
}

const dictSamePOSFixture = `[
  {
    "meta": {"id": "record:1", "stems": ["record"]},
    "hwi": {"hw": "record", "prs": [{"mw": "ˈre-kərd"}]},
    "fl": "noun",
    "def": [{"sseq": [[["sense", {"sn": "1", "dt": [["text", "a piece of evidence"]]}]]]}]
  },
  {
    "meta": {"id": "record:2", "stems": ["record"]},
    "hwi": {"hw": "record"},
    "fl": "noun",
    "def": [{"sseq": [[["sense", {"sn": "2", "dt": [["text", "the best performance"]]}]]]}]
  }
]`

func TestFetch_SamePOSMerged(t *testing.T) {
	dictSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(dictSamePOSFixture))
	}))
	defer dictSrv.Close()

	thesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer thesSrv.Close()

	client := api.NewClient("k", "k",
		api.WithDictBaseURL(dictSrv.URL+"/"),
		api.WithThesBaseURL(thesSrv.URL+"/"),
	)

	entry, err := client.Fetch("record")
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(entry.DefinitionGroups) != 1 {
		t.Fatalf("expected 1 POS group (merged), got %d", len(entry.DefinitionGroups))
	}
	if entry.DefinitionGroups[0].POS != "noun" {
		t.Errorf("POS = %q, want noun", entry.DefinitionGroups[0].POS)
	}
	if len(entry.DefinitionGroups[0].Defs) != 2 {
		t.Errorf("expected 2 merged defs, got %d", len(entry.DefinitionGroups[0].Defs))
	}
}
