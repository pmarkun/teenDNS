package classifier

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

type Catalog struct {
	criteria []Criterion
}

type taxonomyDocument struct {
	Axes []struct {
		Code  string                         `json:"code"`
		Bands map[string][]taxonomyCriterion `json:"bands"`
	} `json:"axes"`
}

type taxonomyCriterion struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type guidanceDocument struct {
	Guidance []struct {
		Rating   string `json:"rating"`
		Criteria []struct {
			ID         string   `json:"id"`
			Definition string   `json:"definition"`
			Signals    []string `json:"signals"`
		} `json:"criteria"`
	} `json:"guidance"`
}

func LoadCatalog(taxonomyReader, guidanceReader io.Reader) (*Catalog, error) {
	var taxonomy taxonomyDocument
	if err := json.NewDecoder(taxonomyReader).Decode(&taxonomy); err != nil {
		return nil, fmt.Errorf("decode taxonomy: %w", err)
	}
	var guidance guidanceDocument
	if err := json.NewDecoder(guidanceReader).Decode(&guidance); err != nil {
		return nil, fmt.Errorf("decode guidance: %w", err)
	}

	type metadata struct{ label, axis, rating string }
	known := make(map[string]metadata)
	for _, axis := range taxonomy.Axes {
		for rating, criteria := range axis.Bands {
			for _, criterion := range criteria {
				if _, exists := known[criterion.ID]; exists {
					return nil, fmt.Errorf("duplicate taxonomy criterion %s", criterion.ID)
				}
				known[criterion.ID] = metadata{criterion.Label, axis.Code, rating}
			}
		}
	}

	criteria := make([]Criterion, 0, 52)
	seen := make(map[string]bool)
	for _, group := range guidance.Guidance {
		for _, item := range group.Criteria {
			meta, ok := known[item.ID]
			if !ok {
				return nil, fmt.Errorf("guidance references unknown criterion %s", item.ID)
			}
			if seen[item.ID] {
				return nil, fmt.Errorf("duplicate guidance criterion %s", item.ID)
			}
			if meta.rating != group.Rating {
				return nil, fmt.Errorf("rating mismatch for %s", item.ID)
			}
			if strings.TrimSpace(item.Definition) == "" {
				return nil, fmt.Errorf("empty definition for %s", item.ID)
			}
			seen[item.ID] = true
			criteria = append(criteria, Criterion{
				ID: item.ID, Label: meta.label, Axis: meta.axis, Rating: meta.rating,
				Definition: item.Definition, Signals: append([]string(nil), item.Signals...),
			})
		}
	}
	if len(criteria) == 0 {
		return nil, fmt.Errorf("guidance contains no criteria")
	}
	sort.Slice(criteria, func(i, j int) bool { return criteria[i].ID < criteria[j].ID })
	return &Catalog{criteria: criteria}, nil
}

func (c *Catalog) Criteria() []Criterion {
	return append([]Criterion(nil), c.criteria...)
}

func (c *Catalog) Questions() map[string]Question {
	questions := make(map[string]Question, len(c.criteria))
	for _, criterion := range c.criteria {
		signals := ""
		if len(criterion.Signals) > 0 {
			signals = " Sinais possíveis: " + strings.Join(criterion.Signals, "; ") + "."
		}
		questions[criterion.ID] = Question{
			Instructions: "Com base somente no estado fornecido, há evidência suficiente para este critério de classificação indicativa? Critério: " + criterion.Definition + signals,
			TrueLabel:    "Há evidência concreta no estado para o critério.",
			FalseLabel:   "O estado não contém evidência suficiente para o critério.",
		}
	}
	return questions
}
