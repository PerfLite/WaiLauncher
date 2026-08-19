package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// ArticleDetails holds parsed and structured content for the In-App reader.
type ArticleDetails struct {
	Title           string `json:"title"`
	TranslatedTitle string `json:"translatedTitle,omitempty"`
	HeroImage       string `json:"heroImage"`
	Author          string `json:"author"`
	Date            string `json:"date"`
	DisplayDate     string `json:"displayDate"`
	Tag             string `json:"tag"`
	Kind            string `json:"kind"`
	Link            string `json:"link"`
	ContentHtml     string `json:"contentHtml"`
	TranslatedHtml  string `json:"translatedHtml,omitempty"`
}

var (
	articleCacheMu sync.RWMutex
	articleCache   = make(map[string]*ArticleDetails)
	trHttpClient   = &http.Client{
		Timeout: 10 * time.Second,
	}
)

var (
	reArticleTitle  = regexp.MustCompile(`(?is)<meta[^>]+property=["']og:title["'][^>]*content=["']([^"']+)["']|<meta[^>]+content=["']([^"']+)["'][^>]*property=["']og:title["']`)
	reArticleImage  = regexp.MustCompile(`(?is)<meta[^>]+property=["']og:image["'][^>]*content=["']([^"']+)["']|<meta[^>]+content=["']([^"']+)["'][^>]*property=["']og:image["']`)
	reArticleAuthor = regexp.MustCompile(`(?is)"author"\s*:\s*\{\s*"@type"\s*:\s*"Person"\s*,\s*"name"\s*:\s*"([^"]+)"`)
	reArticleDate   = regexp.MustCompile(`(?is)<meta[^>]+property=["']article:published_time["'][^>]*content=["']([^"']+)["']`)
)

// GetArticle fetches and parses a minecraft.net article for the in-app reader.
func (l *Launcher) GetArticle(ctx context.Context, rawURL string) (artRes *ArticleDetails, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[GetArticle PANIC] %v\nStack:\n%s", r, string(debug.Stack()))
			retErr = fmt.Errorf("internal parser error: %v", r)
		}
	}()

	if !isMinecraftNet(rawURL) {
		return nil, fmt.Errorf("invalid article URL: %s", rawURL)
	}

	articleCacheMu.RLock()
	if cached, ok := articleCache[rawURL]; ok && cached != nil && cached.ContentHtml != "" {
		articleCacheMu.RUnlock()
		return cached, nil
	}
	articleCacheMu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
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
		return nil, fmt.Errorf("GET %s: status %d", rawURL, resp.StatusCode)
	}

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	htmlStr := string(rawBody)

	art := &ArticleDetails{
		Link: rawURL,
	}

	// Title
	if m := reArticleTitle.FindStringSubmatch(htmlStr); len(m) > 0 {
		if m[1] != "" {
			art.Title = strings.TrimSpace(m[1])
		} else if len(m) > 2 {
			art.Title = strings.TrimSpace(m[2])
		}
	}
	if art.Title == "" {
		if titleTag := regexp.MustCompile(`(?is)<title>(.*?)(?:\||\-|\<)`).FindStringSubmatch(htmlStr); len(titleTag) > 1 {
			art.Title = strings.TrimSpace(titleTag[1])
		}
	}

	// Tag & Kind
	art.Tag, art.Kind = tagFor(art.Title)
	art.Tag = l.T("news.tag." + art.Tag)

	// Hero Image
	if m := reArticleImage.FindStringSubmatch(htmlStr); len(m) > 0 {
		if m[1] != "" {
			art.HeroImage = m[1]
		} else if len(m) > 2 {
			art.HeroImage = m[2]
		}
	}

	// Author
	if m := reArticleAuthor.FindStringSubmatch(htmlStr); len(m) > 1 {
		art.Author = strings.TrimSpace(m[1])
	}
	if art.Author == "" {
		art.Author = "Mojang"
	}

	// Date
	if m := reArticleDate.FindStringSubmatch(htmlStr); len(m) > 1 {
		art.Date = m[1]
		if when := parsePubDate(m[1]); !when.IsZero() {
			art.DisplayDate = newsDate(l.Lang, when)
		}
	}
	if art.DisplayDate == "" {
		if m := regexp.MustCompile(`(?is)"datePublished"\s*:\s*"([^"]+)"`).FindStringSubmatch(htmlStr); len(m) > 1 {
			art.Date = m[1]
			if t, err := time.Parse("2006-01-02T15:04:05.000-0700", m[1]); err == nil {
				art.DisplayDate = newsDate(l.Lang, t)
			} else if t, err := time.Parse(time.RFC3339, m[1]); err == nil {
				art.DisplayDate = newsDate(l.Lang, t)
			}
		}
	}

	// Fast HTML parsing using golang.org/x/net/html
	contentHtml, err := parseArticleBodyHtml(htmlStr)
	if err == nil && contentHtml != "" {
		art.ContentHtml = contentHtml
	}

	articleCacheMu.Lock()
	articleCache[rawURL] = art
	articleCacheMu.Unlock()

	return art, nil
}

// parseArticleBodyHtml extracts clean article elements using standard HTML DOM traversal.
func parseArticleBodyHtml(htmlStr string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return "", err
	}

	// Find node with id="main-content"
	var mainNode *html.Node
	var findMain func(*html.Node)
	findMain = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, attr := range n.Attr {
				if attr.Key == "id" && attr.Val == "main-content" {
					mainNode = n
					return
				}
			}
		}
		for c := n.FirstChild; c != nil && mainNode == nil; c = c.NextSibling {
			findMain(c)
		}
	}
	findMain(doc)

	if mainNode == nil {
		mainNode = doc
	}

	var sb bytes.Buffer

	// Extract meaningful content nodes: p, h2, h3, h4, ul, ol, blockquote, picture, img
	var extractContent func(*html.Node)
	extractContent = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Skip scripts, style, headers, navigation, footers, share widgets
			for _, attr := range n.Attr {
				if attr.Key == "class" {
					v := strings.ToLower(attr.Val)
					if strings.Contains(v, "globalheader") ||
						strings.Contains(v, "globalfooter") ||
						strings.Contains(v, "socialsharea") ||
						strings.Contains(v, "cookie") ||
						strings.Contains(v, "searchbox") ||
						strings.Contains(v, "minecoinsummary") {
						return
					}
				}
			}

			switch n.DataAtom {
			case atom.Script, atom.Style, atom.Nav, atom.Header, atom.Footer, atom.Button:
				return
			case atom.P, atom.H2, atom.H3, atom.H4, atom.Ul, atom.Ol, atom.Blockquote:
				cleanNode(n)
				var buf bytes.Buffer
				if err := html.Render(&buf, n); err == nil {
					s := strings.TrimSpace(buf.String())
					if s != "" && s != "<p></p>" && s != "<p> </p>" {
						sb.WriteString("<div class=\"article-block\">")
						sb.WriteString(s)
						sb.WriteString("</div>\n")
					}
				}
				return
			case atom.Img:
				cleanNode(n)
				var buf bytes.Buffer
				if err := html.Render(&buf, n); err == nil {
					sb.WriteString("<div class=\"article-block\">")
					sb.WriteString(buf.String())
					sb.WriteString("</div>\n")
				}
				return
			case atom.Picture:
				cleanNode(n)
				var buf bytes.Buffer
				if err := html.Render(&buf, n); err == nil {
					sb.WriteString("<div class=\"article-block\">")
					sb.WriteString(buf.String())
					sb.WriteString("</div>\n")
				}
				return
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractContent(c)
		}
	}

	extractContent(mainNode)
	return sb.String(), nil
}

func cleanNode(n *html.Node) {
	if n.Type == html.ElementNode {
		for i, attr := range n.Attr {
			if attr.Key == "src" || attr.Key == "srcset" {
				if strings.HasPrefix(attr.Val, "/") {
					n.Attr[i].Val = "https://www.minecraft.net" + attr.Val
				}
			}
			if attr.Key == "href" {
				if strings.HasPrefix(attr.Val, "/") {
					n.Attr[i].Val = "https://www.minecraft.net" + attr.Val
				}
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		cleanNode(c)
	}
}

// TranslateArticle translates article title and content to the target language (e.g. "ru").
func (l *Launcher) TranslateArticle(ctx context.Context, rawURL string, targetLang string) (artRes *ArticleDetails, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[TranslateArticle PANIC] %v\nStack:\n%s", r, string(debug.Stack()))
			retErr = fmt.Errorf("internal translation error: %v", r)
		}
	}()

	if targetLang == "" {
		targetLang = "ru"
	}

	art, err := l.GetArticle(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	articleCacheMu.RLock()
	if art.TranslatedHtml != "" && art.TranslatedTitle != "" {
		articleCacheMu.RUnlock()
		return art, nil
	}
	articleCacheMu.RUnlock()

	// Translate title
	tTitle := translateTextReliable(ctx, art.Title, targetLang)
	if tTitle != "" {
		art.TranslatedTitle = tTitle
	} else {
		art.TranslatedTitle = art.Title
	}

	// Safely translate all text nodes inside the HTML
	tHtml, err := translateHtmlTextNodes(ctx, art.ContentHtml, targetLang)
	if err == nil && tHtml != "" {
		art.TranslatedHtml = tHtml
	} else {
		art.TranslatedHtml = art.ContentHtml
	}

	articleCacheMu.Lock()
	articleCache[rawURL] = art
	articleCacheMu.Unlock()

	return art, nil
}

// translateHtmlTextNodes safely modifies TextNode data in-place without removing nodes or corrupting DOM pointers.
func translateHtmlTextNodes(ctx context.Context, htmlContent, targetLang string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent, err
	}

	var textNodes []*html.Node
	var originalTexts []string

	var collectTextNodes func(*html.Node)
	collectTextNodes = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.DataAtom == atom.Script || n.DataAtom == atom.Style || n.DataAtom == atom.Code {
				return
			}
		}
		if n.Type == html.TextNode {
			t := strings.TrimSpace(n.Data)
			if len(t) > 1 && hasLetters(t) {
				textNodes = append(textNodes, n)
				originalTexts = append(originalTexts, t)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collectTextNodes(c)
		}
	}
	collectTextNodes(doc)

	translatedTexts := make([]string, len(originalTexts))
	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup

	for i, raw := range originalTexts {
		wg.Add(1)
		go func(idx int, str string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
			defer cancel()

			translatedTexts[idx] = translateTextReliable(cctx, str, targetLang)
		}(i, raw)
	}
	wg.Wait()

	// In-place assignment into text nodes
	for i, node := range textNodes {
		if i < len(translatedTexts) && translatedTexts[i] != "" {
			node.Data = translatedTexts[i]
		}
	}

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return htmlContent, err
	}
	return buf.String(), nil
}

func hasLetters(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

func translateTextReliable(ctx context.Context, text, targetLang string) string {
	clean := strings.TrimSpace(text)
	if len(clean) <= 1 {
		return text
	}

	// For long text chunks, split into sentences so the query stays small and clean
	if len(clean) > 400 {
		sentences := splitSentences(clean)
		var sb strings.Builder
		for i, s := range sentences {
			if i > 0 {
				sb.WriteString(" ")
			}
			tr := translateTextReliable(ctx, s, targetLang)
			sb.WriteString(tr)
		}
		return sb.String()
	}

	// 1. Google Webapp
	if tr, err := translateChunkGoogle(ctx, clean, targetLang); err == nil && tr != "" {
		return tr
	}

	// 2. MyMemory Fallback
	if tr, err := translateChunkMyMemory(ctx, clean, targetLang); err == nil && tr != "" {
		return tr
	}

	return text
}

func translateChunkGoogle(ctx context.Context, text, targetLang string) (string, error) {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return "", nil
	}

	endpoint := fmt.Sprintf("https://translate.google.com/m?sl=auto&tl=%s&hl=%s&q=%s",
		url.QueryEscape(targetLang), url.QueryEscape(targetLang), url.QueryEscape(clean))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := trHttpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil {
		return "", err
	}

	bodyStr := string(body)
	if strings.Contains(bodyStr, "af-error-page") || strings.Contains(bodyStr, "Error 500") || strings.Contains(bodyStr, "<script") {
		return "", fmt.Errorf("error page detected")
	}

	m := regexp.MustCompile(`(?is)<div[^>]+class=["']result-container["'][^>]*>(.*?)</div>`).FindStringSubmatch(bodyStr)
	if len(m) > 1 {
		res := htmlUnescape(strings.TrimSpace(m[1]))
		if !strings.Contains(res, "<style") && !strings.Contains(res, "<script") && !strings.Contains(res, "af-error") {
			return res, nil
		}
	}
	return "", fmt.Errorf("no valid result")
}

func translateChunkMyMemory(ctx context.Context, text, targetLang string) (string, error) {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return "", nil
	}

	endpoint := fmt.Sprintf("https://api.mymemory.translated.net/get?q=%s&langpair=en|%s",
		url.QueryEscape(clean), url.QueryEscape(targetLang))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "WaiLauncher/0.1")

	resp, err := trHttpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil {
		return "", err
	}

	var res struct {
		ResponseData struct {
			TranslatedText string `json:"translatedText"`
		} `json:"responseData"`
		ResponseStatus int `json:"responseStatus"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", err
	}
	if res.ResponseData.TranslatedText != "" {
		out := htmlUnescape(res.ResponseData.TranslatedText)
		if !strings.Contains(out, "MYMEMORY WARNING") {
			return out, nil
		}
	}
	return "", fmt.Errorf("empty translation")
}

func splitSentences(text string) []string {
	var res []string
	parts := strings.Split(text, ". ")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if i < len(parts)-1 && !strings.HasSuffix(p, ".") {
			p += "."
		}
		res = append(res, p)
	}
	if len(res) == 0 {
		return []string{text}
	}
	return res
}

func htmlUnescape(s string) string {
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	return s
}
