package pronunciation

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var audioTemplateRE = regexp.MustCompile(`(?i)\{\{audio\|en\|([^|}]+)(?:\|([^}]+))?\}\}`)
var htmlTagRE = regexp.MustCompile(`<[^>]*>`)

type WiktionaryProvider struct {
	APIURL string
	Client *RetryingClient
}

func NewWiktionaryProvider(client *RetryingClient) *WiktionaryProvider {
	return &WiktionaryProvider{APIURL: "https://en.wiktionary.org/w/api.php", Client: client}
}
func (p *WiktionaryProvider) Name() string { return "wiktionary" }

func (p *WiktionaryProvider) Lookup(ctx context.Context, query Query) (Result, error) {
	params := url.Values{"action": {"parse"}, "page": {query.Text}, "prop": {"wikitext"}, "format": {"json"}, "formatversion": {"2"}}
	var parsed struct {
		Parse struct {
			Wikitext string `json:"wikitext"`
		} `json:"parse"`
		Error any `json:"error"`
	}
	if err := p.getJSON(ctx, p.APIURL+"?"+params.Encode(), &parsed); err != nil {
		return Result{}, err
	}
	if parsed.Parse.Wikitext == "" {
		return Result{}, ErrNotFound
	}
	var filename string
	for _, match := range audioTemplateRE.FindAllStringSubmatch(parsed.Parse.Wikitext, -1) {
		if len(match) > 2 && isBritishDescription(match[1]+" "+match[2]) {
			filename = strings.TrimSpace(match[1])
			break
		}
	}
	if filename == "" {
		return Result{}, ErrNotFound
	}
	commons := "https://commons.wikimedia.org/w/api.php"
	if u, err := url.Parse(p.APIURL); err == nil && (u.Host == "127.0.0.1" || u.Host == "localhost") {
		commons = strings.TrimRight(p.APIURL, "/")
	}
	q := url.Values{"action": {"query"}, "titles": {"File:" + filename}, "prop": {"imageinfo"}, "iiprop": {"url|extmetadata"}, "format": {"json"}, "formatversion": {"2"}}
	var info struct {
		Query struct {
			Pages []struct {
				ImageInfo []struct {
					URL, DescriptionURL string
					ExtMetadata         map[string]struct {
						Value any `json:"value"`
					} `json:"extmetadata"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := p.getJSON(ctx, commons+"?"+q.Encode(), &info); err != nil {
		return Result{}, err
	}
	if len(info.Query.Pages) == 0 || len(info.Query.Pages[0].ImageInfo) == 0 {
		return Result{}, ErrNotFound
	}
	i := info.Query.Pages[0].ImageInfo[0]
	meta := i.ExtMetadata
	return Result{Provider: p.Name(), Locale: query.Locale, AudioURL: i.URL, SourceURL: i.DescriptionURL, LicenseName: metadataString(meta, "LicenseShortName"), LicenseURL: metadataString(meta, "LicenseUrl"), Attribution: cleanHTML(metadataString(meta, "Artist"))}, nil
}

func (p *WiktionaryProvider) getJSON(ctx context.Context, raw string, dst any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	req.Header.Set("User-Agent", "niuniu-education/1.0")
	resp, err := p.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return &UpstreamError{Provider: p.Name(), StatusCode: resp.StatusCode, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode wiktionary: %w", err)
	}
	return nil
}
func isBritishDescription(s string) bool {
	s = strings.ToLower(s)
	for _, k := range []string{"uk", "british", "england", "received pronunciation", "rp", "southern english", "northern english"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
func cleanHTML(s string) string {
	return strings.TrimSpace(html.UnescapeString(htmlTagRE.ReplaceAllString(s, "")))
}

func metadataString(metadata map[string]struct {
	Value any `json:"value"`
}, key string) string {
	value, ok := metadata[key]
	if !ok || value.Value == nil {
		return ""
	}
	if text, ok := value.Value.(string); ok {
		return text
	}
	return fmt.Sprint(value.Value)
}
