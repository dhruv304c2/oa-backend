package agent

import "google.golang.org/genai"

type Agent struct {
	ID      string
	History []*genai.Content
}
