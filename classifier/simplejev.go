package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type SimpleJEV struct {
	Endpoint string
	Model    string
	Token    string
	Client   *http.Client
}

type simpleJEVRequest struct {
	Model     string                       `json:"model"`
	State     PageProfile                  `json:"state"`
	Questions map[string]simpleJEVQuestion `json:"questions"`
}

type simpleJEVQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

type simpleJEVResponse struct {
	Answers map[string]struct {
		Type        string  `json:"type"`
		Noul        float64 `json:"noul"`
		Probability float64 `json:"probability"`
	} `json:"answers"`
}

func (j *SimpleJEV) Evaluate(ctx context.Context, state PageProfile, questions map[string]Question) (map[string]float64, error) {
	if strings.TrimSpace(j.Endpoint) == "" {
		return nil, fmt.Errorf("Simple JEV endpoint is required")
	}
	if strings.TrimSpace(j.Model) == "" {
		return nil, fmt.Errorf("Simple JEV model is required")
	}
	requestQuestions := make(map[string]simpleJEVQuestion, len(questions))
	for id, question := range questions {
		requestQuestions[id] = simpleJEVQuestion{
			Type:         "noul",
			Instructions: question.Instructions,
			Criteria: map[string]string{
				"true":  question.TrueLabel,
				"false": question.FalseLabel,
			},
		}
	}
	payload, err := json.Marshal(simpleJEVRequest{Model: j.Model, State: state, Questions: requestQuestions})
	if err != nil {
		return nil, fmt.Errorf("encode Simple JEV request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, j.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create Simple JEV request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if j.Token != "" {
		request.Header.Set("Authorization", "Bearer "+j.Token)
	}
	client := j.Client
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Simple JEV: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("call Simple JEV: HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var output simpleJEVResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&output); err != nil {
		return nil, fmt.Errorf("decode Simple JEV response: %w", err)
	}
	scores := make(map[string]float64, len(questions))
	for id := range questions {
		answer, ok := output.Answers[id]
		if !ok {
			return nil, fmt.Errorf("Simple JEV response is missing answer %s", id)
		}
		score := answer.Noul
		if answer.Type == "boolean" {
			score = answer.Probability
		}
		if score < 0 || score > 1 {
			return nil, fmt.Errorf("Simple JEV returned invalid score for %s", id)
		}
		scores[id] = score
	}
	return scores, nil
}

func (j *SimpleJEV) Info() ClassifierInfo {
	return ClassifierInfo{Name: "simple-jev", Version: j.Model}
}
