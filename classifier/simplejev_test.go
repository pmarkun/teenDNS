package classifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSimpleJEVEvaluate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", request.Method)
		}
		var input simpleJEVRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if input.Model != "test-model" || input.Questions["A.6.5"].Type != "noul" {
			t.Fatalf("unexpected input: %+v", input)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"answers":{"A.6.5":{"type":"noul","noul":0.91}}}`))
	}))
	defer server.Close()

	engine := &SimpleJEV{Endpoint: server.URL, Model: "test-model"}
	scores, err := engine.Evaluate(context.Background(), PageProfile{URL: "https://example.org"}, map[string]Question{
		"A.6.5": {Instructions: "question", TrueLabel: "yes", FalseLabel: "no"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if scores["A.6.5"] != 0.91 {
		t.Fatalf("unexpected scores: %#v", scores)
	}
}
