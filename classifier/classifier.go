package classifier

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"
)

type URLExtractor interface {
	Extract(context.Context, string) (PageProfile, error)
}

type Classifier struct {
	Catalog   *Catalog
	Extractor URLExtractor
	Engine    Engine
	Threshold float64
	Now       func() time.Time
}

func (c *Classifier) ClassifyURL(ctx context.Context, rawURL string) (Result, error) {
	if c.Catalog == nil || c.Extractor == nil || c.Engine == nil {
		return Result{}, fmt.Errorf("catalog, extractor and engine are required")
	}
	threshold := c.Threshold
	if threshold == 0 {
		threshold = 0.75
	}
	if threshold <= 0 || threshold > 1 {
		return Result{}, fmt.Errorf("threshold must be greater than 0 and at most 1")
	}
	profile, err := c.Extractor.Extract(ctx, rawURL)
	if err != nil {
		return Result{}, err
	}
	scores, err := c.Engine.Evaluate(ctx, profile, c.Catalog.Questions())
	if err != nil {
		return Result{}, err
	}
	criteriaByID := make(map[string]Criterion)
	for _, criterion := range c.Catalog.Criteria() {
		criteriaByID[criterion.ID] = criterion
	}
	matched := make([]CriterionResult, 0)
	maxRating := 0
	for id, score := range scores {
		if score < threshold {
			continue
		}
		criterion, ok := criteriaByID[id]
		if !ok {
			continue
		}
		rating, _ := strconv.Atoi(criterion.Rating)
		if rating > maxRating {
			maxRating = rating
		}
		matched = append(matched, CriterionResult{
			Axis: criterion.Axis, CriterionID: criterion.ID, Label: criterion.Label, Confidence: score,
			Evidence: []Evidence{{
				Kind:        "metadata",
				Description: "Pontuação calculada sobre o perfil textual extraído da página; requer revisão da evidência original.",
				Locator:     profile.FinalURL,
			}},
		})
	}
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].Confidence == matched[j].Confidence {
			return matched[i].CriterionID < matched[j].CriterionID
		}
		return matched[i].Confidence > matched[j].Confidence
	})
	rating := "unknown"
	decision := "observe"
	reasons := []string{"Nenhum critério 14+ superou o limiar; isso não comprova classificação livre ou 12."}
	if maxRating > 0 {
		rating = strconv.Itoa(maxRating)
		decision = "mediate"
		reasons = []string{"Foi detectado ao menos um critério incompatível com a faixa de 12 a 13 anos."}
		if maxRating >= 16 {
			decision = "protect"
		}
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	return Result{
		TaxonomyVersion: TaxonomyVersion,
		ProfileID:       ProfileID,
		Subject:         Subject{URL: profile.FinalURL, Scope: "page"},
		Rating:          rating,
		Criteria:        matched,
		Decision:        decision,
		DecisionReasons: reasons,
		ReviewStatus:    "machine_suggestion",
		ClassifiedAt:    now().UTC(),
		Classifier:      c.Engine.Info(),
	}, nil
}
