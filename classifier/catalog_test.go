package classifier

import (
	"os"
	"testing"
)

func TestLoadProjectCatalog(t *testing.T) {
	taxonomy, err := os.Open("../criteria/age-rating-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	defer taxonomy.Close()
	guidance, err := os.Open("../criteria/priority-guidance-12-13-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	defer guidance.Close()

	catalog, err := LoadCatalog(taxonomy, guidance)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(catalog.Criteria()); got != 52 {
		t.Fatalf("expected 52 priority criteria, got %d", got)
	}
	questions := catalog.Questions()
	if questions["A.6.5"].Instructions == "" {
		t.Fatal("expected a question for suicide criterion")
	}
}
