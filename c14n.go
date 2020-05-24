package c14n

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"

	"github.com/ucarion/c14n/internal/stack"
)

type RawTokenReader interface {
	RawToken() (xml.Token, error)
}

func Canonicalize(r RawTokenReader) ([]byte, error) {
	var s stack.Stack
	buf := bytes.Buffer{}

	var startRootNode *xml.StartElement // the start of the root node, special-cased

loop:
	for {
		token, err := r.RawToken()
		if err != nil {
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			// First, process the name declarations provided in this element. We will
			// need these in order to determine the appropriate namespace URI for a
			// particular local name.
			//
			// In this step, we also determine if we are working with the root of the
			// node-set we are to canonicalize.
			names := map[string]string{}
			for _, attr := range t.Attr {
				if attr.Name.Space == "xmlns" {
					names[attr.Name.Local] = attr.Value
				} else if attr.Name.Space == "" && attr.Name.Local == "xmlns" {
					names[""] = attr.Value
				}
			}

			// Push the names onto the stack. Elements after us are going to need
			// these names in order to determine their attribute sort order,
			// regardless of whether this particular element is part of the node-set
			// to canonicalize.
			s.Push(names)

			// Resolve the element and all of its attributes. We don't actually care
			// about the attributes, but we need the stack to be informed of what
			// namespaces are visibly used.
			s.Get(t.Name.Space)
			for _, attr := range t.Attr {
				if attr.Name.Space != "xmlns" && (attr.Name.Space != "" || attr.Name.Local != "xmlns") {
					s.Get(attr.Name.Space)
				}
			}

			// If we are the root node, then we will need to be rendered at the very
			// end, because you can't render the root node without knowing the set of
			// visibly-used namespaces, which will be determined by later tokens.
			if startRootNode == nil {
				// t is only valid for this iteration of the for loop, we must copy it.
				copyT := t.Copy()
				startRootNode = &copyT
			} else {
				// If we are not the root node, then we render ourselves immediately.
				writeStartElement(&s, &buf, t, false)
			}
		case xml.EndElement:
			// Continuing the part of the spec abridged in the StartElement-handling
			// section:
			//
			// [...] an open angle bracket, a forward slash (/), the element QName,
			// and a close angle bracket.
			//
			// We implement that here.

			if t.Name.Space == "" {
				fmt.Fprintf(&buf, "</%s>", t.Name.Local)
			} else {
				fmt.Fprintf(&buf, "</%s:%s>", t.Name.Space, t.Name.Local)
			}

			// If we are about to pop back onto the root level, then don't pop the
			// stack at all, and instead jump to the special-case handling of
			// startRootNode; no further nodes needed to be rendered.
			if s.Len() == 1 {
				break loop
			}

			// Pop the stack of namespaces.
			s.Pop()
		case xml.CharData:
			// From the spec:
			//
			// Text Nodes- the string value, except all ampersands are replaced by
			// &amp;, all open angle brackets (<) are replaced by &lt;, all closing
			// angle brackets (>) are replaced by &gt;, and all #xD characters are
			// replaced by &#xD;.
			//
			// xml.EscapeText does not implement this, and practice this is a
			// significant problem because it will escape newlines into "&#xA;". So we
			// implement our own replacement here.
			//
			// Also, to clarify: #xD is usually known as "carriage return" (\r).

			// Don't start rendering output until we've reached a StartElement.
			if startRootNode == nil {
				continue
			}

			t = bytes.ReplaceAll(t, amp, escAmp)
			t = bytes.ReplaceAll(t, lt, escLt)
			t = bytes.ReplaceAll(t, gt, escGt)
			t = bytes.ReplaceAll(t, cr, escCr)

			buf.Write(t)
		case xml.ProcInst:
			// From the spec:
			//
			// Processing Instruction (PI) Nodes- The opening PI symbol (<?), the PI
			// target name of the node, a leading space and the string value if it is
			// not empty, and the closing PI symbol (?>). If the string value is
			// empty, then the leading space is not added. Also, a trailing #xA is
			// rendered after the closing PI symbol for PI children of the root node
			// with a lesser document order than the document element, and a leading
			// #xA is rendered before the opening PI symbol of PI children of the root
			// node with a greater document order than the document element.
			//
			// However:
			//
			// The XML declaration, including version number and character encoding is
			// omitted from the canonical form. The encoding is not needed since the
			// canonical form is encoded in UTF-8. The version is not needed since the
			// absence of a version number unambiguously indicates XML 1.0.
			//
			// https://www.w3.org/TR/2001/REC-xml-c14n-20010315#NoXMLDecl
			//
			// We implement this omission by simply checking if the target of the
			// ProcInst is xml.

			// Don't start rendering output until we've reached a StartElement.
			if startRootNode == nil {
				continue
			}

			if t.Target != "xml" {
				fmt.Fprintf(&buf, "<?%s", t.Target)
				if len(t.Inst) > 0 {
					buf.WriteByte(' ')
				}
				buf.Write(t.Inst)
				fmt.Fprintf(&buf, "?>")
			}
		}
	}

	out := bytes.Buffer{}
	writeStartElement(&s, &out, *startRootNode, true)
	out.Write(buf.Bytes())

	return out.Bytes(), nil
}

func writeStartElement(s *stack.Stack, buf *bytes.Buffer, t xml.StartElement, isRoot bool) {
	// From the exclusive c14n spec, which differs here from ordinary c14n spec:
	//
	// A namespace node N with a prefix that does not appear in the
	// InclusiveNamespaces PrefixList is rendered if all of the conditions are
	// met:
	//
	// Its parent element is in the node-set, and
	//
	// it is visibly utilized by its parent element, and
	//
	// the prefix has not yet been rendered by any output ancestor, or the nearest
	// output ancestor of its parent element that visibly utilizes the namespace
	// prefix does not have a namespace node in the node-set with the same
	// namespace prefix and value as N.
	//
	// (https://www.w3.org/TR/xml-exc-c14n/)
	//
	// We implement this by copying over any namespace attribute that is visibly
	// used from this level in the stack, as well as all non-namespace attributes.
	used := s.Used()

	// Remove any namespace attributes from the node. The visibly-used namespaces
	// will determine what we output for namespace nodes.
	attrs := []xml.Attr{}
	for _, attr := range t.Attr {
		if attr.Name.Space != "xmlns" && (attr.Name.Space != "" || attr.Name.Local != "xmlns") {
			attrs = append(attrs, attr)
		}
	}

	// Copy over all visibly-used namespaces into the node.
	for name, uri := range used {
		if name == "" {
			attrs = append(attrs, xml.Attr{
				Name:  xml.Name{Space: "", Local: "xmlns"},
				Value: uri,
			})
		} else {
			attrs = append(attrs, xml.Attr{
				Name:  xml.Name{Space: "xmlns", Local: name},
				Value: uri,
			})
		}
	}

	// Establish a sorted order of attributes using sortAttr, which implements the
	// ordering rules of the c14n spec.
	sortAttr := sortAttr{stack: s, attrs: attrs}
	sort.Sort(sortAttr)

	// Write out the element. From the spec:
	//
	// If the element is in the node-set, then the result is an open angle bracket
	// (<), the element QName, the result of processing the namespace axis, the
	// result of processing the attribute axis, a close angle bracket (>), [...]
	//
	// Where QName is:
	//
	// The QName of a node is either the local name if the namespace prefix string
	// is empty or the namespace prefix, a colon, then the local name of the
	// element. The namespace prefix used in the QName MUST be the same one which
	// appeared in the input document.
	//
	// https://www.w3.org/TR/2001/REC-xml-c14n-20010315#ProcessingModel
	//
	// So here we write out '<' unconditionally, and then write out space:local if
	// there's a space, or just local otherwise.
	//
	// We do not here implement the more complex rules for handling the default
	// namespace.
	if t.Name.Space == "" {
		fmt.Fprintf(buf, "<%s", t.Name.Local)
	} else {
		fmt.Fprintf(buf, "<%s:%s", t.Name.Space, t.Name.Local)
	}

	for _, attr := range sortAttr.attrs {
		// From the spec:
		//
		// Attribute Nodes- a space, the node's QName, an equals sign, an open
		// quotation mark (double quote), the modified string value, and a close
		// quotation mark (double quote). The string value of the node is modified
		// by replacing all ampersands (&) with &amp;, all open angle brackets (<)
		// with &lt;, all quotation mark characters with &quot;, and the whitespace
		// characters #x9, #xA, and #xD, with character references. The character
		// references are written in uppercase hexadecimal with no leading zeroes
		// (for example, #xD is represented by the character reference &#xD;).
		//
		// QName is already described in a comment above.
		//
		// https://www.w3.org/TR/2001/REC-xml-c14n-20010315#ProcessingModel
		//
		// xml.EscapeText does not implement this, and practice this is a
		// significant problem because it will escape single-quotes into "&#x39;".
		// So we implement our own replacement here.
		if attr.Name.Space == "" {
			fmt.Fprintf(buf, " %s=\"", attr.Name.Local)
		} else {
			fmt.Fprintf(buf, " %s:%s=\"", attr.Name.Space, attr.Name.Local)
		}

		val := []byte(attr.Value)
		val = bytes.ReplaceAll(val, amp, escAmp)
		val = bytes.ReplaceAll(val, lt, escLt)
		val = bytes.ReplaceAll(val, quot, escQuot)
		val = bytes.ReplaceAll(val, tab, escTab)
		val = bytes.ReplaceAll(val, nl, escNl)
		val = bytes.ReplaceAll(val, cr, escCr)
		buf.Write(val)

		fmt.Fprint(buf, "\"")
	}

	// Having processed the attributes, we now close out the tag:
	fmt.Fprint(buf, ">")
}

// These are used in handling xml.CharData and xml.StartElement attribute
// values.
var (
	amp    = []byte("&")
	escAmp = []byte("&amp;")
	lt     = []byte("<")
	escLt  = []byte("&lt;")
	gt     = []byte(">")
	escGt  = []byte("&gt;")
	cr     = []byte("\r")
	escCr  = []byte("&#xD;")
)

// These are used exclusively in handling xml.StartElement attribute values.
var (
	quot    = []byte("\"")
	escQuot = []byte("&quot;")
	tab     = []byte("\t")
	escTab  = []byte("&#x9;")
	nl      = []byte("\n")
	escNl   = []byte("&#xA;")
)

type sortAttr struct {
	stack *stack.Stack
	attrs []xml.Attr
}

func (s sortAttr) Len() int {
	return len(s.attrs)
}

func (s sortAttr) Swap(i, j int) {
	s.attrs[i], s.attrs[j] = s.attrs[j], s.attrs[i]
}

func (s sortAttr) Less(i, j int) bool {
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
	if s.attrs[i].Name.Space == "" && s.attrs[i].Name.Local == "xmlns" {
		return true
	}

	if s.attrs[j].Name.Space == "" && s.attrs[j].Name.Local == "xmlns" {
		return false
	}

	// Namespace nodes go first. If one is a namespace node and the other isn't,
	// then it goes first.
	if s.attrs[i].Name.Space == "xmlns" && s.attrs[j].Name.Space != "xmlns" {
		return true
	}

	if s.attrs[i].Name.Space != "xmlns" && s.attrs[j].Name.Space == "xmlns" {
		return false
	}

	// Break ties between two namespace nodes by their local name.
	if s.attrs[i].Name.Space == "xmlns" && s.attrs[j].Name.Space == "xmlns" {
		return s.attrs[i].Name.Local < s.attrs[j].Name.Local
	}

	// Finally:
	//
	// "An element's attribute nodes are sorted lexicographically with namespace
	// URI as the primary key and local name as the secondary key (an empty
	// namespace URI is lexicographically least)."
	//
	// This just means: sort by Space first, break ties by Local.
	spaceI := s.stack.Get(s.attrs[i].Name.Space)
	spaceJ := s.stack.Get(s.attrs[j].Name.Space)
	if spaceI != spaceJ {
		return spaceI < spaceJ
	}

	return s.attrs[i].Name.Local < s.attrs[j].Name.Local
}
