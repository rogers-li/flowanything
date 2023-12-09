package api

import (
	"flow-anything/model"
	"fmt"
)

const (
	ProtocolTypeHttp = "http"
)

type Protocol struct {
	Type string
	Data *HttpProtocol
}

type HttpProtocol struct {
	Method      string
	Url         string
	ContentType string
	Header      []model.Field
	Cookie      []model.Field
}

type Message struct {
	Request  []model.Field
	Response []model.Field
}

type Api struct {
	ApiID string
	Protocol
	Message
	ApiRequester Requester
}

func (a *Api) GetHttpProtocol() (*HttpProtocol, error) {
	if a.Protocol.Type != ProtocolTypeHttp {
		return nil, fmt.Errorf("not http protocol")
	}
	return a.Data, nil
}
