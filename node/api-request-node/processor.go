package api_request_node

import (
	"flow-anything/api"
	"flow-anything/eventbus"
	"flow-anything/fastflow"
	"flow-anything/model"
	"flow-anything/utils"
)

// ApiRequestNodeProcessor api调用节点的处理器
type ApiRequestNodeProcessor struct {
	fastflow.CommonProcessor
	apiPool *api.Pool
}

func NewApiRequestNodeProcessor(apiPool *api.Pool) *ApiRequestNodeProcessor {
	return &ApiRequestNodeProcessor{
		apiPool: apiPool,
	}
}

func (a *ApiRequestNodeProcessor) BeforeExecute(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus) (bool, error) {
	return true, nil
}

func (a *ApiRequestNodeProcessor) Execute(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus) (bool, error) {
	var nodeData ApiRequestNodeData
	err := utils.ReConstruct(node.NodeData, &nodeData)
	if err != nil {
		return false, err
	}
	req, err := a.buildRequest(flowCtx, nodeData.RequestFields)
	if err != nil {
		return false, err
	}
	targetApi, err := a.apiPool.GetApi(nodeData.ApiID)
	if err != nil {
		return false, err
	}
	resp, err := targetApi.ApiRequester.Request(targetApi, req)
	a.recordRequestLog(flowCtx, nodeData.ApiID, req, resp, err)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (a *ApiRequestNodeProcessor) AfterExecute(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus) error {
	return a.CommonProcessor.AfterExecute(flowCtx, node, bus)
}

func (a *ApiRequestNodeProcessor) ExecuteFailed(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus, err error) {
	a.CommonProcessor.ExecuteFailed(flowCtx, node, bus, err)
}

// 根据配置的请求字段，组装请求报文
func (a *ApiRequestNodeProcessor) buildRequest(flowCtx *fastflow.FlowCtx, RequestFields []model.FieldWithInitExpression) (map[string]interface{}, error) {
	req := make(map[string]interface{})
	runtimeCtx := flowCtx.GetFlowRuntimeCtx()
	for _, field := range RequestFields {
		name := field.FieldName
		value, err := utils.GetValByPath(runtimeCtx, field.FieldExpression)
		if err != nil {
			return nil, err
		}
		req[name] = value
	}
	return req, nil
}

// 记录api调用的详细数据
func (a *ApiRequestNodeProcessor) recordRequestLog(flowCtx *fastflow.FlowCtx, ApiID string, req map[string]interface{}, resp interface{}, err error) {
	apiLog := NewApiLog(req, resp, err)
	resultCtx := flowCtx.GetAndInitResultCtx(apiResultCtxKey, NewApiResult)
	apiResult := resultCtx.(ApiResult)
	apiResult.lock.Lock()
	apiResult.ApiLogs[ApiID] = apiLog
	apiResult.lock.Unlock()
}
