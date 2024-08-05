package api

import "flow-anything/variable"

type Config struct {
	Define
	RequestTmpParams  TmpParams
	Request           variable.Field
	RequesterConfig   RequesterConfig
	ResponseTmpParams TmpParams
	Response          variable.Field
}

type TmpParams map[string]variable.Field

type Define struct {
	ApiName string
	ApiID   string
	ApiDesc string
}

type RequesterConfig struct {
}
