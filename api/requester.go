package api

type Requester interface {
	Request(api *Api, request map[string]interface{}) (response interface{}, err error)
}
