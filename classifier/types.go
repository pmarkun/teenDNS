// Package classifier extracts a bounded profile from a web page and evaluates
// it against the teenDNS age-rating taxonomy using a typed decision engine.
package classifier

import (
	"context"
	"time"
)

const (
	TaxonomyVersion = "1.0.0"
	ProfileID       = "br-12-13-v1"
)

type PageProfile struct {
	URL         string   `json:"url"`
	FinalURL    string   `json:"final_url"`
	Domain      string   `json:"domain"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Text        string   `json:"visible_text,omitempty"`
	Links       []string `json:"link_domains,omitempty"`
}

type Subject struct {
	URL   string `json:"url"`
	Scope string `json:"scope"`
}

type Evidence struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Locator     string `json:"locator,omitempty"`
}

type CriterionResult struct {
	Axis        string     `json:"axis"`
	CriterionID string     `json:"criterion_id"`
	Label       string     `json:"label"`
	Confidence  float64    `json:"confidence"`
	Evidence    []Evidence `json:"evidence"`
}

type ClassifierInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Result struct {
	TaxonomyVersion string            `json:"taxonomy_version"`
	ProfileID       string            `json:"profile_id"`
	Subject         Subject           `json:"subject"`
	Rating          string            `json:"rating"`
	Criteria        []CriterionResult `json:"criteria"`
	Decision        string            `json:"decision"`
	DecisionReasons []string          `json:"decision_reasons,omitempty"`
	ReviewStatus    string            `json:"review_status"`
	ClassifiedAt    time.Time         `json:"classified_at"`
	Classifier      ClassifierInfo    `json:"classifier"`
}

type Criterion struct {
	ID         string
	Label      string
	Axis       string
	Rating     string
	Definition string
	Signals    []string
}

type Question struct {
	Instructions string
	TrueLabel    string
	FalseLabel   string
}

type Engine interface {
	Evaluate(ctx context.Context, state PageProfile, questions map[string]Question) (map[string]float64, error)
	Info() ClassifierInfo
}
