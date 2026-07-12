package pronunciation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type stubProvider struct {
	name   string
	result Result
	err    error
	calls  *atomic.Int32
}

func (s stubProvider) Name() string { return s.name }
func (s stubProvider) Lookup(context.Context, Query) (Result, error) {
	if s.calls != nil {
		s.calls.Add(1)
	}
	return s.result, s.err
}

func TestCompositeContinuesAfter429(t *testing.T) {
	var second atomic.Int32
	p := NewCompositeProvider(stubProvider{name: "limited", err: &UpstreamError{Provider: "limited", StatusCode: 429}}, stubProvider{name: "backup", result: Result{AudioURL: "ok"}, calls: &second})
	r, err := p.Lookup(context.Background(), Query{Text: "hospital", Locale: "en-GB"})
	if err != nil || r.Provider != "backup" || second.Load() != 1 {
		t.Fatalf("unexpected: %#v %v", r, err)
	}
}

func TestRetryingClientHonors429(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		w.WriteHeader(200)
	}))
	defer s.Close()
	c := NewRetryingClient(s.Client(), 0, 1)
	c.BaseBackoff = time.Millisecond
	req, _ := http.NewRequest(http.MethodGet, s.URL, nil)
	resp, err := c.Do(req)
	if err != nil || resp.StatusCode != 200 || calls.Load() != 2 {
		t.Fatalf("status/calls: %v %d", err, calls.Load())
	}
	resp.Body.Close()
}

func TestFreeDictionarySelectsUK(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"phonetic":"/x/","phonetics":[{"text":"/us/","audio":"https://x/a-us.mp3"},{"text":"/uk/","audio":"https://x/a-uk.mp3","sourceUrl":"source"}]}]`))
	}))
	defer s.Close()
	p := NewFreeDictionaryProvider(NewRetryingClient(s.Client(), 0, 0))
	p.BaseURL = s.URL
	r, err := p.Lookup(context.Background(), Query{Text: "a", Locale: "en-GB"})
	if err != nil || r.AudioURL != "https://x/a-uk.mp3" || r.Phonetic != "/uk/" {
		t.Fatalf("unexpected: %#v %v", r, err)
	}
}
