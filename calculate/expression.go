package calculate

import (
	"fmt"
	"github.com/Knetic/govaluate"
)

type Expression struct {
}

func (e *Expression) ValByIndex(exp string, ctx map[string]interface{}) (interface{}, error) {
	return nil, nil
}

func (e *Expression) ValByBool(exp string, ctx map[string]interface{}) (bool, error) {
	expression, err := govaluate.NewEvaluableExpression(exp)
	if err != nil {
		return false, err
	}
	result, err := expression.Evaluate(ctx)
	if err != nil {
		return false, err
	}
	b, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("not bool expression[%s]", exp)
	}
	return b, nil
}

func (e *Expression) ValByOperator(exp string, ctx map[string]interface{}) (interface{}, error) {
	return nil, nil
}

func (e *Expression) ValByFunction(exp string, ctx map[string]interface{}) (interface{}, error) {
	return nil, nil
}

func (e *Expression) ValByTemplate(exp string, ctx map[string]interface{}) (interface{}, error) {
	return nil, nil
}

func (e *Expression) ValAssign(exp string, ctx map[string]interface{}) error {
	return nil
}

func (e *Expression) LogicExecute(exp string, ctx map[string]interface{}) error {
	return nil
}
