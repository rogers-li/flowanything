package api

import "fmt"

type Pool struct {
	store map[string]*Api
}

func NewApiPool() *Pool {
	return &Pool{store: map[string]*Api{}}
}

func (a *Pool) GetApi(apiID string) (*Api, error) {
	api, ok := a.store[apiID]
	if !ok {
		return nil, fmt.Errorf("api not found [%s]", apiID)
	}
	return api, nil
}
