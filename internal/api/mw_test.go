package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/230pe/lazydict/internal/api"
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
	if entry.FunctionalLabel != "noun" {
		t.Errorf("FL = %q, want %q", entry.FunctionalLabel, "noun")
	}
	if len(entry.Definitions) == 0 {
		t.Error("expected at least one definition")
	}
	if entry.Definitions[0] != "1. a thing constituting evidence" {
		t.Errorf("def = %q", entry.Definitions[0])
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
