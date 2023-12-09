package calculate_node

import "flow-anything/model"

const (
	NodeTypeCalculate = "calculate" // api请求节点的节点类型
)

type CalculateNodeData struct {
	Function string                          `json:"function"`
	Input    []model.FieldWithInitExpression `json:"input"`
	Output   []model.FieldWithInitExpression `json:"output"`
}
