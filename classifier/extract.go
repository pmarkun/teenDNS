package classifier

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"
)

const (
	defaultMaxBody = int64(2 << 20)
	defaultMaxText = 12_000
)

type Extractor struct {
	Client  *http.Client
	MaxBody int64
	MaxText int
}

func NewExtractor(allowPrivate bool) *Extractor {
	return &Extractor{
		Client:  safeHTTPClient(allowPrivate),
		MaxBody: defaultMaxBody,
		MaxText: defaultMaxText,
	}
}

func (e *Extractor) Extract(ctx context.Context, rawURL string) (PageProfile, error) {
	parsed, err := validateURL(rawURL)
	if err != nil {
		return PageProfile{}, err
	}
	client := e.Client
	if client == nil {
		client = safeHTTPClient(false)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return PageProfile{}, fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("User-Agent", "teenDNS-classifier/0.1")
	request.Header.Set("Accept", "text/html,application/xhtml+xml")

	response, err := client.Do(request)
	if err != nil {
		return PageProfile{}, fmt.Errorf("fetch page: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return PageProfile{}, fmt.Errorf("fetch page: unexpected HTTP status %d", response.StatusCode)
	}
	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
		return PageProfile{}, fmt.Errorf("unsupported content type %q", contentType)
	}
	maxBody := e.MaxBody
	if maxBody <= 0 {
		maxBody = defaultMaxBody
	}
	limited := io.LimitReader(response.Body, maxBody+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return PageProfile{}, fmt.Errorf("read page: %w", err)
	}
	if int64(len(payload)) > maxBody {
		return PageProfile{}, fmt.Errorf("page exceeds %d byte limit", maxBody)
	}
	document, err := html.Parse(strings.NewReader(string(payload)))
	if err != nil {
		return PageProfile{}, fmt.Errorf("parse HTML: %w", err)
	}
	finalURL := response.Request.URL
	profile := PageProfile{
		URL:      parsed.String(),
		FinalURL: finalURL.String(),
		Domain:   strings.ToLower(finalURL.Hostname()),
	}
	profile.Title, profile.Description, profile.Text, profile.Links = extractDocument(document, finalURL, e.MaxText)
	return profile, nil
}

func extractDocument(document *html.Node, base *url.URL, maxText int) (string, string, string, []string) {
	if maxText <= 0 {
		maxText = defaultMaxText
	}
	var title, description string
	var text strings.Builder
	links := make(map[string]struct{})
	var walk func(*html.Node, bool)
	walk = func(node *html.Node, hidden bool) {
		if node.Type == html.ElementNode {
			tag := strings.ToLower(node.Data)
			if tag == "script" || tag == "style" || tag == "noscript" || tag == "svg" || tag == "template" {
				hidden = true
			}
			if tag == "meta" {
				name, content := attribute(node, "name"), attribute(node, "content")
				property := attribute(node, "property")
				if description == "" && (strings.EqualFold(name, "description") || strings.EqualFold(property, "og:description")) {
					description = cleanText(content)
				}
			}
			if tag == "a" {
				href := attribute(node, "href")
				if target, err := base.Parse(href); err == nil && (target.Scheme == "http" || target.Scheme == "https") {
					if host := strings.ToLower(target.Hostname()); host != "" {
						links[host] = struct{}{}
					}
				}
			}
		}
		if !hidden && node.Type == html.TextNode {
			value := cleanText(node.Data)
			if value != "" {
				if node.Parent != nil && node.Parent.Type == html.ElementNode && strings.EqualFold(node.Parent.Data, "title") && title == "" {
					title = value
				}
				if text.Len() < maxText {
					if text.Len() > 0 {
						text.WriteByte(' ')
					}
					writeLimited(&text, value, maxText-text.Len())
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, hidden)
		}
	}
	walk(document, false)
	linkList := make([]string, 0, len(links))
	for link := range links {
		linkList = append(linkList, link)
	}
	sort.Strings(linkList)
	if len(linkList) > 100 {
		linkList = linkList[:100]
	}
	return title, description, text.String(), linkList
}

func writeLimited(output *strings.Builder, value string, maxBytes int) {
	if maxBytes <= 0 {
		return
	}
	if len(value) <= maxBytes {
		output.WriteString(value)
		return
	}
	for _, character := range value {
		encoded := string(character)
		if len(encoded) > maxBytes {
			break
		}
		output.WriteString(encoded)
		maxBytes -= len(encoded)
	}
}

func attribute(node *html.Node, key string) string {
	for _, item := range node.Attr {
		if strings.EqualFold(item.Key, key) {
			return item.Val
		}
	}
	return ""
}

func cleanText(value string) string {
	return strings.Join(strings.FieldsFunc(value, unicode.IsSpace), " ")
}

func validateURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("URL scheme must be http or https")
	}
	if parsed.Hostname() == "" {
		return nil, errors.New("URL host is required")
	}
	if parsed.User != nil {
		return nil, errors.New("URL credentials are not allowed")
	}
	return parsed, nil
}

func safeHTTPClient(allowPrivate bool) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		// Proxies could resolve a public-looking request to an internal target and
		// bypass the dial-time address checks below.
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("split destination: %w", err)
			}
			addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("resolve %s: %w", host, err)
			}
			if len(addresses) == 0 {
				return nil, fmt.Errorf("resolve %s: no addresses", host)
			}
			for _, candidate := range addresses {
				if !allowPrivate && unsafeIP(candidate.IP) {
					continue
				}
				return dialer.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
			}
			return nil, fmt.Errorf("destination %s resolves only to private or unsafe addresses", host)
		},
		ForceAttemptHTTP2:     true,
		ResponseHeaderTimeout: 8 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		_, err := validateURL(request.URL.String())
		return err
	}
	return client
}

func unsafeIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified()
}
