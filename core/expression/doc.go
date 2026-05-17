// Package expression evaluates runtime-neutral mapping expressions.
//
// It centralizes context reads, constant assignment, node-output reads, and
// context writes so workflow nodes, agent nodes, and tool wrappers can share
// one mapping semantics.
package expression
