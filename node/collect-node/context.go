package collect_node

import (
	"sync"
)

const (
	collectResultCtxKey = "variable_result" // api_request节点写入流程结果数据中的这个key
)

type Result struct {
	Variables map[string]interface{}
	lock      sync.Locker
}
