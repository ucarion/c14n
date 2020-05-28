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

type RawTokenReader interface {
	RawToken() (xml.Token, error)
}

func Canonicalize(r RawTokenReader) ([]byte, error) {
	var knownNames stack.Stack
	var renderedNames stack.Stack
	var buf bytes.Buffer

	for {
		t, err := r.RawToken()
		if err != nil {
			if err == io.EOF {
				fmt.Println(string(buf.Bytes()))
				return nil, io.ErrUnexpectedEOF
			}

			return nil, err
		}

		switch t := t.(type) {
		case xml.StartElement:
			names := map[string]string{}
			visiblyUsedNames := map[string]struct{}{}

			visiblyUsedNames[t.Name.Space] = struct{}{}
			for _, attr := range t.Attr {
				if name, ok := getNamespace(attr); ok {
					names[name] = attr.Value
				} else {
					visiblyUsedNames[attr.Name.Space] = struct{}{}
				}
			}

			previousDefaultNamespace, _ := knownNames.Get("")
			knownNames.Push(names)

			namesToRender := map[string]struct{}{}
			for name, uri := range knownNames.GetAll() {
				shouldRender := false

				if name == "" && uri == "" {
					_, visiblyUsed := visiblyUsedNames[""]
					declaredValue, declared := names[""]
					_, rendered := renderedNames.Get("")

					shouldRender = visiblyUsed && (!declared || declaredValue != previousDefaultNamespace) && rendered
				} else {
					_, visiblyUsed := visiblyUsedNames[name]
					renderedValue, rendered := renderedNames.Get(name)

					shouldRender = visiblyUsed && (!rendered || renderedValue != uri)
				}

				if shouldRender {
					namesToRender[name] = struct{}{}
				}
			}

			attrsToRender := []xml.Attr{}
			for _, attr := range t.Attr {
				if _, ok := getNamespace(attr); !ok {
					attrsToRender = append(attrsToRender, attr)
				}
			}

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

			// Establish a sorted order of attributes using sortAttr, which implements the
			// ordering rules of the c14n spec.
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
