package trace

import "sort"

type mutableNode struct {
	span     Span
	children []*mutableNode
}

func BuildTree(spans []Span) []SpanTreeNode {
	nodes := make(map[string]*mutableNode, len(spans))
	order := make([]string, 0, len(spans))
	for _, span := range spans {
		spanCopy := span
		nodes[span.SpanID] = &mutableNode{span: spanCopy}
		order = append(order, span.SpanID)
	}
	sort.SliceStable(order, func(i, j int) bool {
		left := nodes[order[i]].span
		right := nodes[order[j]].span
		if left.StartedAt.Equal(right.StartedAt) {
			return left.SpanID < right.SpanID
		}
		return left.StartedAt.Before(right.StartedAt)
	})

	roots := []*mutableNode{}
	for _, spanID := range order {
		node := nodes[spanID]
		parent, ok := nodes[node.span.ParentSpanID]
		if node.span.ParentSpanID != "" && ok {
			parent.children = append(parent.children, node)
			continue
		}
		roots = append(roots, node)
	}
	out := make([]SpanTreeNode, 0, len(roots))
	for _, root := range roots {
		out = append(out, immutableTree(root))
	}
	return out
}

func immutableTree(node *mutableNode) SpanTreeNode {
	out := SpanTreeNode{Span: node.span, Children: make([]SpanTreeNode, 0, len(node.children))}
	for _, child := range node.children {
		out.Children = append(out.Children, immutableTree(child))
	}
	return out
}
