package flow_anything

import (
	"flow-anything/api"
	"flow-anything/eventbus"
	"flow-anything/fastflow"
	api_request_node "flow-anything/node/api-request-node"
	calculate_node "flow-anything/node/calculate-node"
	collect_node "flow-anything/node/collect-node"
)

type Runner struct {
	flowRunner *fastflow.Runner
}

func NewRunner() *Runner {
	bus := eventbus.NewDefaultEventBus()
	flowRunner := fastflow.NewRunner(bus)
	apiPool := api.NewApiPool()
	apiNodeProcessor := api_request_node.NewApiRequestNodeProcessor(apiPool)
	flowRunner.AddNodeProcessor(api_request_node.NodeTypeApiRequest, apiNodeProcessor)
	flowRunner.AddNodeProcessor(calculate_node.NodeTypeCalculate, calculate_node.NewCalculateNodeProcessor())
	flowRunner.AddNodeProcessor(collect_node.NodeTypeCollect, collect_node.NewCollectNodeProcessor())

	runner := &Runner{
		flowRunner: flowRunner,
	}
	return runner
}

func (r *Runner) CallFlow(flow *fastflow.Flow, input map[string]interface{}) (map[string]interface{}, error) {
	return r.flowRunner.Run(flow, input)
}
