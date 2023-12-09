package fastflow

import "flow-anything/eventbus"

type NodeExecutor struct {
	nodeProcessorFactory *NodeProcessorFactory
}

func NewNodeExecutor() *NodeExecutor {
	nodeProcessorFactory := NewNodeProcessorFactory()
	return &NodeExecutor{nodeProcessorFactory: nodeProcessorFactory}
}

func (n *NodeExecutor) ExecuteNode(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus) error {
	nodeProcessor, err := n.nodeProcessorFactory.GetNodeProcessor(node.NodeType)
	if err != nil {
		return err
	} else {
		toNext, err := nodeProcessor.BeforeExecute(flowCtx, node, bus)
		if toNext {
			toNext, err = nodeProcessor.Execute(flowCtx, node, bus)
			if toNext {
				err = nodeProcessor.AfterExecute(flowCtx, node, bus)
			}
		}
		if err != nil {
			nodeProcessor.ExecuteFailed(flowCtx, node, bus, err)
		}
		return nil
	}
}

func (n *NodeExecutor) AddNodeProcessor(nodeType string, nodeProcessor INodeProcessor) {
	n.nodeProcessorFactory.RegistryNode(nodeType, nodeProcessor)
}
