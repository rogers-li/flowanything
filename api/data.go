package api

type DataContext struct {
	input       map[string]interface{}
	requestTmp  map[string]interface{}
	request     map[string]interface{}
	response    map[string]interface{}
	responseTmp map[string]interface{}
	output      map[string]interface{}
}
