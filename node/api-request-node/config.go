package api_request_node

import "flow-anything/model"

const (
	NodeTypeApiRequest = "api_request" // api请求节点的节点类型
)

// ApiRequestNodeData api调用节点配置
type ApiRequestNodeData struct {
	ApiID         string                          `json:"api_id"`         // api id
	RequestFields []model.FieldWithInitExpression `json:"request_fields"` // api请求参数定义
}
