package fast_flow_test

import (
	"bytes"
	"encoding/json"
	"flow-anything/eventbus"
	fast_flow_test "flow-anything/example/fast-flow-test/debug-node"
	"flow-anything/fastflow"
	"flow-anything/hook"
	"fmt"
	"os"
	"testing"
)

func TestFastFlow(t *testing.T) {
	bus := eventbus.NewDefaultEventBus()
	fastFlowRunner := fastflow.NewRunner(bus)
	hook.NewFlowTracer(bus).StartTracing()
	fastFlowRunner.AddNodeProcessor(fast_flow_test.NodeTypeDebug, fast_flow_test.NewDebugNodeProcessor())
	flow := getFlow("/flow.json")
	result, err := fastFlowRunner.Run(flow, nil)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(result)
}

func getFlow(path string) *fastflow.Flow {
	pwd, _ := os.Getwd()
	b, err := os.ReadFile(pwd + path)
	if err != nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	var document *fastflow.Flow
	err = decoder.Decode(&document)
	if err != nil {
		return nil
	}
	return document
}
