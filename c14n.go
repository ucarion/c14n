// Package c14n implements Exclusive Canonical XML canonicalization (commonly
// abbbreviated "c14n").
//
// https://www.w3.org/TR/xml-exc-c14n/
package c14n

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"

	"github.com/ucarion/c14n/internal/sortattr"
	"github.com/ucarion/c14n/internal/stack"
)

// RawTokenReader is similar to xml.TokenReader, but is expected to return
// tokens whose namespaces are not automatically resolved.
//
// xml.Decoder implements this interface and complies with its contract.
type RawTokenReader interface {
	// RawToken returns the next raw token in the reader, or an error. RawToken
	// must not return a token and an error simultaneously.
	RawToken() (xml.Token, error)
}

// Canonicalize returns the canonicalized representation of a sequence of raw
// XML tokens. In particular, it implements Exclusive Canonical XML, the
// recommended canonicalization scheme for the SAML protocol.
//
// Canonicalize will render the first root-level attribute in the input token
// sequence. Any leading character data, comments, or directives will be
// skipped.
//
// The input stream is not checked for correctness. Canonicalize's behavior is
// undefined if given unbalanced tokens or other incorrect XML input.
func Canonicalize(r RawTokenReader) ([]byte, error) {
	var knownNames stack.Stack    // a mapping of all declared namespaces in the input
	var renderedNames stack.Stack // a mapping of all declared namespaces in the output
	var buf bytes.Buffer          // the output buffer

	for {
		t, err := r.RawToken()
		if err != nil {
			if err == io.EOF {
				return nil, io.ErrUnexpectedEOF
			}

			return nil, err
		}

		switch t := t.(type) {
		case xml.StartElement:
			names := map[string]string{}              // the names declared by this element
			visiblyUsedNames := map[string]struct{}{} // the names visibly used by this element

			visiblyUsedNames[t.Name.Space] = struct{}{}
			for _, attr := range t.Attr {
				if name, ok := getNamespace(attr); ok {
					names[name] = attr.Value
				} else {
					visiblyUsedNames[attr.Name.Space] = struct{}{}
				}
			}

			// Note the previous value of the default namespace. This needs to be
			// special-cased because the c14n spec special-cases the case of xmlns="".
			previousDefaultNamespace, _ := knownNames.Get("")

			// Push all the names declared by this element onto the input stack. We
			// will use this to determine what namespaces to put on the output stack.
			knownNames.Push(names)

			namesToRender := map[string]struct{}{} // namespaces we will want to output
			for name, uri := range knownNames.GetAll() {
				shouldRender := false

				// xmlns="" is special-cased.
				if name == "" && uri == "" {
					// Per the spec, from the non-normative but clearer "constrained
					// implementation":
					//
					// Render xmlns="" if and only if all of the conditions are met:
					//
					// The default namespace is visibly utilized by the immediate parent
					// element node, or the default prefix token is present in
					// InclusiveNamespaces PrefixList, and
					//
					// the element does not have a namespace node in the node-set
					// declaring a value for the default namespace, and
					//
					// the default namespace prefix is present in the dictionary
					// ns_rendered.
					//
					// ns_rendered corresponds to renderedNames in this code.
					_, visiblyUsed := visiblyUsedNames[""]
					declaredValue, declared := names[""]
					_, rendered := renderedNames.Get("")

					shouldRender = visiblyUsed && (!declared || declaredValue != previousDefaultNamespace) && rendered
				} else {
					// Again from the spec:
					//
					// Render each namespace node if and only if all of the conditions are
					// met:
					//
					// it is visibly utilized by the immediate parent element or one of
					// its attributes, or is present in InclusiveNamespaces PrefixList,
					// and
					//
					// its prefix and value do not appear in ns_rendered.
					_, visiblyUsed := visiblyUsedNames[name]
					renderedValue, rendered := renderedNames.Get(name)

					shouldRender = visiblyUsed && (!rendered || renderedValue != uri)
				}

				if shouldRender {
					namesToRender[name] = struct{}{}
				}
			}

			// attrsToRender is the set of attributes we'll render. The order doesn't
			// matter yet, we'll sort them later.
			attrsToRender := []xml.Attr{}
			for _, attr := range t.Attr {
				// Render all non-namespace ndoes.
				if _, ok := getNamespace(attr); !ok {
					attrsToRender = append(attrsToRender, attr)
				}
			}

			// renderedNameValues contains the names we're going to render, in a
			// format we can push onto renderedNames.
			renderedNameValues := map[string]string{}
			for name := range namesToRender {
				uri, _ := knownNames.Get(name)
				renderedNameValues[name] = uri

				if name == "" {
					attrsToRender = append(attrsToRender, xml.Attr{
						Name:  xml.Name{Space: "", Local: "xmlns"},
						Value: uri,
					})
				} else {
					attrsToRender = append(attrsToRender, xml.Attr{
						Name:  xml.Name{Space: "xmlns", Local: name},
						Value: uri,
					})
				}
			}

			renderedNames.Push(renderedNameValues)

			// Establish a sorted order of attributes using SortAttr, which implements
			// the ordering rules of the c14n spec.
			sortAttr := sortattr.SortAttr{Stack: &knownNames, Attrs: attrsToRender}
			sort.Sort(sortAttr)

			// Write out the element. From the spec:
			//
			// If the element is in the node-set, then the result is an open angle
			// bracket (<), the element QName, the result of processing the namespace
			// axis, the result of processing the attribute axis, a close angle
			// bracket (>), [...]
			//
			// Where QName is:
			//
			// The QName of a node is either the local name if the namespace prefix
			// string is empty or the namespace prefix, a colon, then the local name
			// of the element. The namespace prefix used in the QName MUST be the same
			// one which appeared in the input document.
			//
			// https://www.w3.org/TR/2001/REC-xml-c14n-20010315#ProcessingModel
			//
			// So here we write out '<' unconditionally, and then write out
			// space:local if there's a space, or just local otherwise.
			//
			// We do not here implement the more complex rules for handling the
			// default namespace.
			if t.Name.Space == "" {
				fmt.Fprintf(&buf, "<%s", t.Name.Local)
			} else {
				fmt.Fprintf(&buf, "<%s:%s", t.Name.Space, t.Name.Local)
			}

			for _, attr := range sortAttr.Attrs {
				// From the spec:
				//
				// Attribute Nodes- a space, the node's QName, an equals sign, an open
				// quotation mark (double quote), the modified string value, and a close
				// quotation mark (double quote). The string value of the node is
				// modified by replacing all ampersands (&) with &amp;, all open angle
				// brackets (<) with &lt;, all quotation mark characters with &quot;,
				// and the whitespace characters #x9, #xA, and #xD, with character
				// references. The character references are written in uppercase
				// hexadecimal with no leading zeroes (for example, #xD is represented
				// by the character reference &#xD;).
				//
				// QName is already described in a comment above.
				//
				// https://www.w3.org/TR/2001/REC-xml-c14n-20010315#ProcessingModel
				//
				// xml.EscapeText does not implement this, and practice this is a
				// significant problem because it will escape single-quotes into
				// "&#x39;". So we implement our own replacement here.
				if attr.Name.Space == "" {
					fmt.Fprintf(&buf, " %s=\"", attr.Name.Local)
				} else {
					fmt.Fprintf(&buf, " %s:%s=\"", attr.Name.Space, attr.Name.Local)
				}

				val := []byte(attr.Value)
				val = bytes.ReplaceAll(val, amp, escAmp)
				val = bytes.ReplaceAll(val, lt, escLt)
				val = bytes.ReplaceAll(val, quot, escQuot)
				val = bytes.ReplaceAll(val, tab, escTab)
				val = bytes.ReplaceAll(val, nl, escNl)
				val = bytes.ReplaceAll(val, cr, escCr)
				buf.Write(val)

				fmt.Fprint(&buf, "\"")
			}

			// Having processed the attributes, we now close out the tag:
			fmt.Fprint(&buf, ">")
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

			knownNames.Pop()
			renderedNames.Pop()

			if knownNames.Len() == 0 {
				return buf.Bytes(), nil
			}
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
			if knownNames == nil {
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
			if knownNames == nil {
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
}

// getNamespace gets the namespace declared by this attribute, and whether it's
// a namespace-declaring attribute.
func getNamespace(attr xml.Attr) (string, bool) {
	if attr.Name.Space == "" && attr.Name.Local == "xmlns" {
		return "", true
	}

	if attr.Name.Space == "xmlns" {
		return attr.Name.Local, true
	}

	return "", false
}

// These are used in handling xml.CharData and xml.StartElement attribute
// values.
var (
	amp     = []byte("&")
	escAmp  = []byte("&amp;")
	lt      = []byte("<")
	escLt   = []byte("&lt;")
	gt      = []byte(">")
	escGt   = []byte("&gt;")
	cr      = []byte("\r")
	escCr   = []byte("&#xD;")
	quot    = []byte("\"")
	escQuot = []byte("&quot;")
	tab     = []byte("\t")
	escTab  = []byte("&#x9;")
	nl      = []byte("\n")
	escNl   = []byte("&#xA;")
)
