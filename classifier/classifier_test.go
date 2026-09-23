package classifier

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeExtractor struct{ profile PageProfile }

func (f fakeExtractor) Extract(context.Context, string) (PageProfile, error) { return f.profile, nil }

type fakeEngine struct{ scores map[string]float64 }

func (f fakeEngine) Evaluate(context.Context, PageProfile, map[string]Question) (map[string]float64, error) {
	return f.scores, nil
}
func (f fakeEngine) Info() ClassifierInfo { return ClassifierInfo{Name: "fake", Version: "test"} }

func testCatalog(t *testing.T) *Catalog {
	t.Helper()
	taxonomy := `{"axes":[{"code":"violence","bands":{"16":[{"id":"A.6.5","label":"Suicídio"}]}}]}`
	guidance := `{"guidance":[{"rating":"16","criteria":[{"id":"A.6.5","definition":"Representação de suicídio.","signals":["tentativa","método"]}]}]}`
	catalog, err := LoadCatalog(strings.NewReader(taxonomy), strings.NewReader(guidance))
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func TestClassifyURLProtectsOnRating16(t *testing.T) {
	service := &Classifier{
		Catalog:   testCatalog(t),
		Extractor: fakeExtractor{PageProfile{FinalURL: "https://example.org/page"}},
		Engine:    fakeEngine{map[string]float64{"A.6.5": 0.91}},
		Threshold: 0.75,
		Now:       func() time.Time { return time.Date(2026, 9, 23, 18, 0, 0, 0, time.UTC) },
	}
	result, err := service.ClassifyURL(context.Background(), "https://example.org/page")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rating != "16" || result.Decision != "protect" || len(result.Criteria) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestClassifyURLKeepsUnknownBelowThreshold(t *testing.T) {
	service := &Classifier{
		Catalog:   testCatalog(t),
		Extractor: fakeExtractor{PageProfile{FinalURL: "https://example.org/page"}},
		Engine:    fakeEngine{map[string]float64{"A.6.5": 0.2}},
		Threshold: 0.75,
	}
	result, err := service.ClassifyURL(context.Background(), "https://example.org/page")
	if err != nil {
		t.Fatal(err)
	}
	if result.Rating != "unknown" || result.Decision != "observe" || len(result.Criteria) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
