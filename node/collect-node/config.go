package collect_node

import (
	"flow-anything/model"
)

const (
	NodeTypeCollect = "collect_variable" // api请求节点的节点类型
)

type CollectNodeData struct {
	CollectFields []CollectField `json:"fields"`
}

type CollectField struct {
	Source model.FieldWithInitExpression `json:"source"`
	Target model.FieldWithInitExpression `json:"target"`
}
