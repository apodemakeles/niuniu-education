package pronunciation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type FreeDictionaryProvider struct {
	BaseURL string
	Client  *RetryingClient
}

func NewFreeDictionaryProvider(client *RetryingClient) *FreeDictionaryProvider {
	return &FreeDictionaryProvider{BaseURL: "https://api.dictionaryapi.dev/api/v2/entries/en", Client: client}
}

func (p *FreeDictionaryProvider) Name() string { return "free_dictionary" }

func (p *FreeDictionaryProvider) Lookup(ctx context.Context, query Query) (Result, error) {
	u := strings.TrimRight(p.BaseURL, "/") + "/" + url.PathEscape(strings.TrimSpace(query.Text))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Result{}, err
	}
	resp, err := p.Client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Result{}, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return Result{}, &UpstreamError{Provider: p.Name(), StatusCode: resp.StatusCode, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	var entries []struct {
		Phonetic  string `json:"phonetic"`
		Phonetics []struct {
			Text, Audio, SourceURL string
			License                struct{ Name, URL string } `json:"license"`
		} `json:"phonetics"`
		SourceURLs []string                   `json:"sourceUrls"`
		License    struct{ Name, URL string } `json:"license"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return Result{}, fmt.Errorf("decode free dictionary: %w", err)
	}
	for _, entry := range entries {
		for _, ph := range entry.Phonetics {
			if ph.Audio == "" || (strings.EqualFold(query.Locale, "en-GB") && !isBritishAudio(ph.Audio)) {
				continue
			}
			audio := ph.Audio
			if strings.HasPrefix(audio, "//") {
				audio = "https:" + audio
			}
			source := ph.SourceURL
			if source == "" && len(entry.SourceURLs) > 0 {
				source = entry.SourceURLs[0]
			}
			licenseName, licenseURL := ph.License.Name, ph.License.URL
			if licenseName == "" {
				licenseName, licenseURL = entry.License.Name, entry.License.URL
			}
			phonetic := ph.Text
			if phonetic == "" {
				phonetic = entry.Phonetic
			}
			return Result{Provider: p.Name(), Locale: query.Locale, Phonetic: phonetic, AudioURL: audio, SourceURL: source, LicenseName: licenseName, LicenseURL: licenseURL}, nil
		}
	}
	return Result{}, ErrNotFound
}

func isBritishAudio(raw string) bool {
	s := strings.ToLower(raw)
	return strings.Contains(s, "-uk.") || strings.Contains(s, "_uk.") || strings.Contains(s, "-gb.") || strings.Contains(s, "_gb.") || strings.Contains(s, "en-uk") || strings.Contains(s, "en_gb")
}
