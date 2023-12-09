package fastflow

const (
	EventTypeStartFlow     = "StartFlow"
	EventTypeEndFlow       = "EndFlow"
	EventTypeNextNode      = "NextNode"
	EventTypeNodeCompleted = "NodeCompleted"
	EventTypeNodeFailed    = "NodeFailed"
	EventTypeFlowFailed    = "FlowFailed"
)

type FlowEventData struct {
	flowInstanceId string
	SourceNodeInfo
	EventData interface{}
}

// CreateFlowEventData todo 这里event相关结构体有点乱，后续封装个FlowEventDataVisitor统一管理
func CreateFlowEventData(node *Node, eventData interface{}, flowInstanceId string) FlowEventData {
	nodeInfo := SourceNodeInfo{}
	if node != nil {
		nodeInfo.nodeId = node.NodeID
	}
	return FlowEventData{
		flowInstanceId: flowInstanceId,
		SourceNodeInfo: nodeInfo,
		EventData:      eventData,
	}
}

type SourceNodeInfo struct {
	nodeId string
}

type NodeBeginEventData struct {
	nodeId string
}

type NodeCompleteEventData struct {
	NodeId string
}

type NodeFailedEventData struct {
	nodeId    string
	failedMsg string
}

type NextNodeEventData struct {
	FromNodeId string
	NextNodeId string
}

type StartFlowEventData struct {
	input map[string]interface{}
}

type EndFlowEventData struct {
}
