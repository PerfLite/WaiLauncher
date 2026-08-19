package launcher

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

// newsFeedURL is the official minecraft.net RSS; it stays fresh, unlike the
// frozen launchercontent.mojang.com feed.
const newsFeedURL = "https://www.minecraft.net/en-us/feeds/community-content/rss"

// NewsEntry is one article card for the UI.
type NewsEntry struct {
	Title       string `json:"title"`
	Tag         string `json:"tag"`
	Kind        string `json:"kind"` // css class: "" | "event" | "patch"
	Date        string `json:"date"`
	DisplayDate string `json:"displayDate"` // "2 января 2026"
	Text        string `json:"text"`
	Image       string `json:"image"`
	Link        string `json:"link"`
}

type rssFeed struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	AtomLink    struct {
		Href string `xml:"href,attr"`
	} `xml:"http://www.w3.org/2005/Atom link"`
}

// GetNews returns fresh minecraft.net articles with cover images scraped from
// each article's og:image meta tag. Cached for 1h with stale fallback.
// The cache stores language-neutral entries; display fields are localized
// per the launcher language at read time.
func (l *Launcher) GetNews(ctx context.Context, refresh bool) ([]NewsEntry, error) {
	cache := filepath.Join(l.CacheDir(), "news.json")
	if !refresh {
		if st, err := os.Stat(cache); err == nil && time.Since(st.ModTime()) < time.Hour {
			if entries := readNewsCache(cache); entries != nil {
				return l.localizeNews(entries), nil
			}
		}
	}
	feed, err := fetchRSS(ctx)
	if err != nil {
		if stale := readNewsCache(cache); stale != nil {
			return l.localizeNews(stale), nil
		}
		return nil, err
	}
	items := feed.Channel.Items
	if len(items) > 12 {
		items = items[:12]
	}
	images := fetchArticleImages(ctx, items)

	entries := make([]NewsEntry, 0, len(items))
	for i, it := range items {
		link := it.AtomLink.Href
		if link == "" {
			continue
		}
		tag, kind := tagFor(it.Title)
		title := strings.TrimSpace(it.Title)
		text := cleanText(it.Description)
		if text == "" || strings.EqualFold(text, title) {
			text = newsMoreSentinel
		}
		entries = append(entries, NewsEntry{
			Title: title,
			Tag:   tag,
			Kind:  kind,
			Date:  it.PubDate,
			Text:  text,
			Image: images[i],
			Link:  link,
		})
	}
	if len(entries) == 0 {
		if stale := readNewsCache(cache); stale != nil {
			return l.localizeNews(stale), nil
		}
	}
	if data, err := json.Marshal(entries); err == nil {
		os.WriteFile(cache, data, 0o644)
	}
	return l.localizeNews(entries), nil
}

// newsMoreSentinel marks entries whose teaser must come from the i18n table.
const newsMoreSentinel = "@more"

// localizeNews fills per-language display fields (tag, date, fallback teaser).
func (l *Launcher) localizeNews(entries []NewsEntry) []NewsEntry {
	for i := range entries {
		e := &entries[i]
		e.Tag = l.T("news.tag." + e.Tag)
		if when := parsePubDate(e.Date); !when.IsZero() {
			e.DisplayDate = newsDate(l.Lang, when)
		}
		if e.Text == newsMoreSentinel {
			e.Text = l.T("news.more")
		}
	}
	return entries
}

func readNewsCache(path string) []NewsEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []NewsEntry
	if json.Unmarshal(data, &entries) != nil || len(entries) == 0 {
		return nil
	}
	return entries
}

// fetchRSS downloads the feed. The server declares UTF-16, so the body is
// transcoded to UTF-8 and the stale XML declaration is stripped.
func fetchRSS(ctx context.Context) (*rssFeed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, newsFeedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "WaiLauncher/0.1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", newsFeedURL, resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	raw = utf16ToUTF8(raw)
	if i := indexClose(raw); i >= 0 {
		raw = raw[i+2:]
	}
	var feed rssFeed
	if err := xml.Unmarshal(raw, &feed); err != nil {
		return nil, err
	}
	if len(feed.Channel.Items) == 0 {
		return nil, errNoNews
	}
	return &feed, nil
}

var errNoNews = fmt.Errorf("GET %s: empty feed", newsFeedURL)

// utf16ToUTF8 converts a UTF-16 payload (LE or BE, BOM optional — the feed
// server sends LE without BOM) to UTF-8.
func utf16ToUTF8(b []byte) []byte {
	le := false
	switch {
	case len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE:
		le = true
		b = b[2:]
	case len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF: // BE with BOM
		b = b[2:]
	case len(b) >= 2 && b[0] != 0 && b[1] == 0: // LE without BOM
		le = true
	case len(b) >= 2 && b[0] == 0 && b[1] != 0: // BE without BOM
	default:
		return b
	}
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		if le {
			u = append(u, uint16(b[i])|uint16(b[i+1])<<8)
		} else {
			u = append(u, uint16(b[i])<<8|uint16(b[i+1]))
		}
	}
	return []byte(string(utf16.Decode(u)))
}

// indexClose finds the end of the "<?xml ... ?>" declaration.
func indexClose(b []byte) int {
	if len(b) < 2 || b[0] != '<' || b[1] != '?' {
		return -1
	}
	for i := 0; i+1 < len(b); i++ {
		if b[i] == '?' && b[i+1] == '>' {
			return i
		}
	}
	return -1
}

var pubDateLayouts = []string{
	time.RFC1123Z,
	"Mon, 02 Jan 2006 15:04:05 Z",
	time.RFC1123,
}

func parsePubDate(s string) time.Time {
	for _, layout := range pubDateLayouts {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t
		}
	}
	return time.Time{}
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// cleanText strips tags/entities leftovers and trims to a card-sized snippet.
func cleanText(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) > 220 {
		return string(r[:217]) + "…"
	}
	return s
}

// tagFor derives the neutral tag key and style from the article title.
func tagFor(title string) (string, string) {
	t := strings.ToLower(title)
	switch {
	case strings.Contains(t, "snapshot") || strings.Contains(t, "preview") || strings.Contains(t, "changelog"):
		return "snapshot", "patch"
	case strings.Contains(t, "marketplace") || strings.Contains(t, "realms"):
		return "services", "event"
	case strings.Contains(t, "java"):
		return "java", ""
	default:
		return "news", ""
	}
}

var ogImageRe = regexp.MustCompile(`<meta[^>]+property=["']og:image["'][^>]*content=["']([^"']+)["']|<meta[^>]+content=["']([^"']+)["'][^>]*property=["']og:image["']`)

// isMinecraftNet allowlists article URLs: only https on minecraft.net hosts.
func isMinecraftNet(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && (u.Host == "minecraft.net" || strings.HasSuffix(u.Host, ".minecraft.net"))
}

// fetchArticleImages pulls each article page and extracts its og:image,
// 8 pages at a time. Missing images stay "".
func fetchArticleImages(ctx context.Context, items []rssItem) []string {
	out := make([]string, len(items))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, it := range items {
		if !isMinecraftNet(it.AtomLink.Href) {
			continue
		}
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			cctx, cancel := context.WithTimeout(ctx, 6*time.Second)
			defer cancel()
			out[i] = ogImage(cctx, url)
		}(i, it.AtomLink.Href)
	}
	wg.Wait()
	return out
}

func ogImage(ctx context.Context, url string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "WaiLauncher/0.1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	// og:image sits in <head>; the first 64 KB are plenty.
	head, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return ""
	}
	m := ogImageRe.FindSubmatch(head)
	if m == nil {
		return ""
	}
	if len(m[1]) > 0 {
		return string(m[1])
	}
	return string(m[2])
}

var ruMonths = [...]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

var enMonths = [...]string{
	"January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

// newsDate formats "2 января 2026" (ru) or "January 2, 2026" (en).
func newsDate(lang string, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	if lang == "en" {
		return enMonths[t.Month()-1] + " " + strconv.Itoa(t.Day()) + ", " + strconv.Itoa(t.Year())
	}
	return strconv.Itoa(t.Day()) + " " + ruMonths[t.Month()-1] + " " + strconv.Itoa(t.Year())
}
