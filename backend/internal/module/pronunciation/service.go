package pronunciation

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/apodemakeles/niuniu-education/backend/internal/module/wordlibrary"
	provider "github.com/apodemakeles/niuniu-education/backend/internal/platform/pronunciation"
)

type WordReader interface {
	Get(context.Context, string) (wordlibrary.Word, error)
}
type Response struct {
	WordID      string `json:"wordId"`
	Locale      string `json:"locale"`
	Provider    string `json:"provider"`
	Phonetic    string `json:"phonetic"`
	AudioURL    string `json:"audioUrl"`
	SourceURL   string `json:"sourceUrl,omitempty"`
	LicenseName string `json:"licenseName,omitempty"`
	LicenseURL  string `json:"licenseUrl,omitempty"`
	Attribution string `json:"attribution,omitempty"`
}
type flight struct {
	done chan struct{}
	rec  Record
	err  error
}
type Service struct {
	store       *Store
	words       WordReader
	provider    provider.Provider
	downloader  *provider.RetryingClient
	root        string
	negativeTTL time.Duration
	mu          sync.Mutex
	flights     map[string]*flight
}

func NewService(store *Store, words WordReader, p provider.Provider, downloader *provider.RetryingClient, root string, negativeTTL time.Duration) *Service {
	return &Service{store: store, words: words, provider: p, downloader: downloader, root: root, negativeTTL: negativeTTL, flights: map[string]*flight{}}
}

func (s *Service) Resolve(ctx context.Context, wordID, locale string) (Record, error) {
	if r, err := s.store.Get(ctx, wordID, locale); err == nil {
		if r.Status == "ready" {
			if _, e := os.Stat(r.FilePath); e == nil {
				return r, nil
			}
		}
		if r.Status == "missing" && time.Now().Before(r.RetryAfter) {
			return Record{}, provider.ErrNotFound
		}
	}
	key := wordID + "|" + locale
	s.mu.Lock()
	if f, ok := s.flights[key]; ok {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return Record{}, ctx.Err()
		case <-f.done:
			return f.rec, f.err
		}
	}
	f := &flight{done: make(chan struct{})}
	s.flights[key] = f
	s.mu.Unlock()
	f.rec, f.err = s.resolveFresh(ctx, wordID, locale)
	close(f.done)
	s.mu.Lock()
	delete(s.flights, key)
	s.mu.Unlock()
	return f.rec, f.err
}
func (s *Service) resolveFresh(ctx context.Context, wordID, locale string) (Record, error) {
	w, err := s.words.Get(ctx, wordID)
	if err != nil {
		return Record{}, err
	}
	result, err := s.provider.Lookup(ctx, provider.Query{Text: w.Text, Locale: locale, Phonetic: w.Phonetic})
	if errors.Is(err, provider.ErrNotFound) {
		_ = s.store.UpsertMissing(ctx, wordID, locale, time.Now().Add(s.negativeTTL))
		return Record{}, err
	}
	if err != nil {
		return Record{}, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, result.AudioURL, nil)
	req.Header.Set("User-Agent", "niuniu-education/1.0 (educational pronunciation cache)")
	resp, err := s.downloader.Do(req)
	if err != nil {
		return Record{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Record{}, &provider.UpstreamError{Provider: result.Provider, StatusCode: resp.StatusCode, RetryAfter: retryAfter(resp.Header.Get("Retry-After"))}
	}
	limited := io.LimitReader(resp.Body, 5<<20+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return Record{}, err
	}
	if len(data) < 100 || len(data) > 5<<20 {
		return Record{}, fmt.Errorf("发音音频大小异常: %d", len(data))
	}
	mime := strings.Split(resp.Header.Get("Content-Type"), ";")[0]
	if mime == "" {
		mime = http.DetectContentType(data)
	}
	ext := ".mp3"
	if strings.Contains(mime, "wav") || strings.Contains(mime, "wave") {
		ext = ".wav"
	} else if strings.Contains(mime, "ogg") {
		ext = ".ogg"
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(w.Text)+"|"+locale+"|"+result.Provider+"|"+result.AudioURL)))[:24]
	dir := filepath.Join(s.root, locale)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Record{}, err
	}
	path := filepath.Join(dir, hash+ext)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return Record{}, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return Record{}, err
	}
	r := Record{WordID: wordID, Locale: locale, Status: "ready", Provider: result.Provider, Phonetic: result.Phonetic, FilePath: path, MIMEType: mime, SourceURL: result.SourceURL, LicenseName: result.LicenseName, LicenseURL: result.LicenseURL, Attribution: result.Attribution}
	if err := s.store.UpsertReady(ctx, r); err != nil {
		return Record{}, err
	}
	return r, nil
}
func retryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if n, err := time.ParseDuration(v + "s"); err == nil {
		return n
	}
	return 0
}
