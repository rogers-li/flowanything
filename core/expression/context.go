package expression

import "fmt"

type MapContext struct {
	Data map[string]any
}

func NewMapContext(data map[string]any) *MapContext {
	if data == nil {
		data = map[string]any{}
	}
	return &MapContext{Data: data}
}

func (c *MapContext) Read(path string) (any, bool) {
	return ReadPath(c.Data, path)
}

func (c *MapContext) Write(path string, value any) error {
	if c.Data == nil {
		c.Data = map[string]any{}
	}
	if path == "" || path == "$" {
		return fmt.Errorf("%w: cannot replace root context", ErrInvalidPath)
	}
	return WritePath(c.Data, path, value)
}
