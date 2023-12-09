package fastflow

const (
	nodeTypeExclusive = "exclusive_gateway"
	nodeTypeParallel  = "parallel_gateway"
)

// todo 这里数据结构有点乱，需要再梳理下
type Flow struct {
	RootNode string           `json:"root_node"`
	Nodes    map[string]*Node `json:"nodes"`
}

type Node struct {
	NodeType   string      `json:"node_type"`
	NodeID     string      `json:"node_id"`
	NodeData   interface{} `json:"node_data"`
	Downstream Stream      `json:"downstream"`
}

// ExclusiveNodeData 对应于Node.NodeData，是exclusive node的具体数据结构
type ExclusiveNodeData struct {
	Conditions       []Condition `json:"conditions"`
	DefaultCondition Condition   `json:"default_condition"`
}

type Condition struct {
	Expression string           `json:"expression"`
	Downstream Stream           `json:"downstream"`
	Assign     []VariableAssign `json:"assign"`
}

type VariableAssign struct {
	AssignExpression string `json:"assign_expression"`
}

type Stream struct {
	FromNodeId string `json:"from_node_id"`
	ToNodeId   string `json:"to_node_id"`
}

// ParallelNodeData 对应于Node.NodeData，是parallel node的具体数据结构
type ParallelNodeData struct {
	Downstream []Stream       `json:"downstream"`
	Upstream   []Stream       `json:"upstream"`
	Assign     VariableAssign `json:"assign"`
}
