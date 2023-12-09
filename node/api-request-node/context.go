package api_request_node

import (
	"sync"
	"time"
)

const (
	apiResultCtxKey = "api_result" // api_request节点写入流程结果数据中的这个key
)

// ApiResult 用于记录api调用的详细信息，和需要返回的字段
// 比如，如果需要调用api1,api2,api3，并且返回api1.field1,api2.field1,api3.field1，那么：
// ApiLogs记录api1,api2,api3三个api的请求和响应字段
// CollectFields会记录api1.field1,api2.field1,api3.field1三个字段
// ApiLogs有两个作用：
// 1、记录api的调用的返回值，供后续其他api使用，例如，api2的请求字段来源于api1的返回字段
// 2、记录api调用的详细数据，用于上层应用展示api的调用链路详情
type ApiResult struct {
	ApiLogs       map[string]ApiLog
	CollectFields map[string]interface{}
	lock          sync.Locker
}

func NewApiResult() interface{} {
	return ApiResult{
		ApiLogs:       map[string]ApiLog{},
		CollectFields: map[string]interface{}{},
		lock:          &sync.RWMutex{},
	}
}

// ApiLog api调用详情记录
type ApiLog struct {
	err       error
	Timestamp int64
	Request   map[string]interface{} // 请求
	Response  interface{}            // 返回
}

func NewApiLog(req map[string]interface{}, resp interface{}, err error) ApiLog {
	return ApiLog{
		err:       err,
		Timestamp: time.Now().UnixMilli(),
		Request:   req,
		Response:  resp,
	}
}
