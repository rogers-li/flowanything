package workflow

const (
	NodeTypeTransform = "workflow.transform"
	NodeTypeConnector = "workflow.connector"
	NodeTypeTool      = "workflow.tool"
	NodeTypeAgent     = "workflow.agent"
)

// NodeRuntime bundles platform capability ports for workflow node executors.
type NodeRuntime struct {
	Connectors ConnectorInvoker
	Tools      ToolInvoker
	Agents     AgentRunner
	Transforms *TransformRegistry
}

func NewNodeRuntime() NodeRuntime {
	return NodeRuntime{Transforms: NewDefaultTransformRegistry()}
}
