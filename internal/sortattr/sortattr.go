package sortattr

import (
	"encoding/xml"

	"github.com/ucarion/c14n/internal/stack"
)

// SortAttr can sort attributes in compliance with the c14n specification.
type SortAttr struct {
	Stack *stack.Stack
	Attrs []xml.Attr
}

// Len implements Sort.
func (s SortAttr) Len() int {
	return len(s.Attrs)
}

// Swap implements Sort.
func (s SortAttr) Swap(i, j int) {
	s.Attrs[i], s.Attrs[j] = s.Attrs[j], s.Attrs[i]
}

// Less implements Sort.
func (s SortAttr) Less(i, j int) bool {
	// Many comments in this function are copied from:
	//
	// https://www.w3.org/TR/2001/REC-xml-c14n-20010315#DocumentOrder

	// The spec states:
	//
	// "Namespace nodes have a lesser document order position than attribute
	// nodes."
	//
	// And:
	//
	// "An element's namespace nodes are sorted lexicographically by local name
	// (the default namespace node, if one exists, has no local name and is
	// therefore lexicographically least)."
	//
	// It follows that the very first node is the default namespace node. Let's
	// handle those first:
	if s.Attrs[i].Name.Space == "" && s.Attrs[i].Name.Local == "xmlns" {
		return true
	}

	if s.Attrs[j].Name.Space == "" && s.Attrs[j].Name.Local == "xmlns" {
		return false
	}

	// Namespace nodes go first. If one is a namespace node and the other isn't,
	// then it goes first.
	if s.Attrs[i].Name.Space == "xmlns" && s.Attrs[j].Name.Space != "xmlns" {
		return true
	}

	if s.Attrs[i].Name.Space != "xmlns" && s.Attrs[j].Name.Space == "xmlns" {
		return false
	}

	// Break ties between two namespace nodes by their local name.
	if s.Attrs[i].Name.Space == "xmlns" && s.Attrs[j].Name.Space == "xmlns" {
		return s.Attrs[i].Name.Local < s.Attrs[j].Name.Local
	}

	// Finally:
	//
	// "An element's attribute nodes are sorted lexicographically with namespace
	// URI as the primary key and local name as the secondary key (an empty
	// namespace URI is lexicographically least)."
	//
	// This just means: sort by Space first, break ties by Local.
	spaceI, _ := s.Stack.Get(s.Attrs[i].Name.Space)
	spaceJ, _ := s.Stack.Get(s.Attrs[j].Name.Space)
	if spaceI != spaceJ {
		return spaceI < spaceJ
	}

	return s.Attrs[i].Name.Local < s.Attrs[j].Name.Local
}
