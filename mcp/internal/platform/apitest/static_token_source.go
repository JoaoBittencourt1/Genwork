package apitest

import (
	"context"
	"sync"
)

// StaticTokenSource hands out its tokens in order, repeating the last one, and records which were invalidated.
type StaticTokenSource struct {
	mutex       sync.Mutex
	tokens      []string
	served      int
	Err         error
	Invalidated []string
}

func NewStaticTokenSource(tokens ...string) *StaticTokenSource {
	return &StaticTokenSource{tokens: tokens}
}

func (source *StaticTokenSource) AccessToken(context.Context) (string, error) {
	source.mutex.Lock()
	defer source.mutex.Unlock()
	if source.Err != nil {
		return "", source.Err
	}
	index := min(source.served, len(source.tokens)-1)
	source.served++
	return source.tokens[index], nil
}

func (source *StaticTokenSource) Invalidate(rejectedAccessToken string) {
	source.mutex.Lock()
	defer source.mutex.Unlock()
	source.Invalidated = append(source.Invalidated, rejectedAccessToken)
}
