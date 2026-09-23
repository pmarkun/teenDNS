package classifier

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestExtractDocument(t *testing.T) {
	document, err := html.Parse(strings.NewReader(`
		<html><head><title> Página de teste </title><meta name="description" content="Uma descrição clara"></head>
		<body><main>Texto visível <a href="https://outside.example/path">link</a></main>
		<script>conteúdo oculto</script><style>também oculto</style></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	base, _ := url.Parse("https://example.org/page")
	title, description, text, links := extractDocument(document, base, 1000)
	if title != "Página de teste" || description != "Uma descrição clara" {
		t.Fatalf("unexpected metadata: %q %q", title, description)
	}
	if !strings.Contains(text, "Texto visível") || strings.Contains(text, "conteúdo oculto") {
		t.Fatalf("unexpected visible text: %q", text)
	}
	if len(links) != 1 || links[0] != "outside.example" {
		t.Fatalf("unexpected links: %#v", links)
	}
}

func TestDefaultExtractorBlocksLoopback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("<html><body>private</body></html>"))
	}))
	defer server.Close()
	if _, err := NewExtractor(false).Extract(context.Background(), server.URL); err == nil {
		t.Fatal("expected loopback URL to be rejected")
	}
}

func TestValidateURL(t *testing.T) {
	for _, rawURL := range []string{"file:///etc/passwd", "https://user:pass@example.org", "https:///missing"} {
		if _, err := validateURL(rawURL); err == nil {
			t.Fatalf("expected %q to be rejected", rawURL)
		}
	}
	if _, err := validateURL("https://example.org/path"); err != nil {
		t.Fatal(err)
	}
}

func TestUnsafeIP(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "10.0.0.1", "169.254.1.1", "::1", "fc00::1"} {
		if !unsafeIP(net.ParseIP(value)) {
			t.Fatalf("expected %s to be unsafe", value)
		}
	}
	if unsafeIP(net.ParseIP("1.1.1.1")) {
		t.Fatal("expected public IP to be allowed")
	}
}
