package api

import (
	"bytes"
	"context"
	"encoding/json"
	"flow-anything/utils"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

type HttpRequester struct {
	Requester
}

func (h *HttpRequester) Request(api *Api, req map[string]interface{}) (response interface{}, err error) {
	httpProtocol, err := api.GetHttpProtocol()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	switch httpProtocol.ContentType {
	case "application/json":
		switch httpProtocol.Method {
		case "POST", "GET":
			var reader io.Reader
			if len(api.Message.Request) > 0 {
				jsonByte, err := json.Marshal(req)
				if err != nil {
					return nil, err
				}
				reader = bytes.NewReader(jsonByte)
			}
			request, _ := http.NewRequestWithContext(ctx, httpProtocol.Method, httpProtocol.Url, reader)
			if err = h.buildHttpHeader(api, ctx, request); err != nil {
				return nil, err
			}
			request.Header.Add("Content-Type", httpProtocol.ContentType)
			if err = h.buildHttpCookie(api, ctx, request); err != nil {
				return nil, err
			}
			return h.doHttpRequest(api, ctx, request)
		}
	case "":
		switch httpProtocol.Method {
		case "GET":
			request, _ := http.NewRequestWithContext(ctx, "GET", httpProtocol.Url, nil)
			if err = h.buildHttpHeader(api, ctx, request); err != nil {
				return nil, err
			}
			if err = h.buildHttpCookie(api, ctx, request); err != nil {
				return nil, err
			}
			return h.doHttpRequest(api, ctx, request)
		}
	}
	return nil, nil
}

func (h *HttpRequester) buildHttpHeader(api *Api, ctx context.Context, request *http.Request) error {
	httpInfo, _ := api.GetHttpProtocol()
	for _, headerExp := range httpInfo.Header {
		request.Header.Add(headerExp.FieldName, utils.ToString(headerExp.DefaultVal))
	}
	return nil
}

func (h *HttpRequester) buildHttpCookie(api *Api, ctx context.Context, request *http.Request) error {
	httpInfo, _ := api.GetHttpProtocol()
	for _, cookieExp := range httpInfo.Cookie {
		request.AddCookie(&http.Cookie{Name: cookieExp.FieldName, Value: utils.ToString(cookieExp.DefaultVal), Expires: time.Now().Add(time.Minute), HttpOnly: true})
	}
	return nil
}

func (h *HttpRequester) doHttpRequest(api *Api, ctx context.Context, request *http.Request) (interface{}, error) {
	httpResponse, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close()
	buf, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, err
	}
	if len(buf) == 0 {
		return nil, nil
	}
	var response interface{}
	dec := json.NewDecoder(bytes.NewBuffer(buf))
	dec.UseNumber()
	err = dec.Decode(&response)
	if err != nil {
		return nil, err
	}
	return response, nil
}
