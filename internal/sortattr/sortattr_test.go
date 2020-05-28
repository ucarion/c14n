package sortattr_test

import (
	"encoding/xml"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/ucarion/c14n/internal/sortattr"
	"github.com/ucarion/c14n/internal/stack"
)

func TestSortAttr(t *testing.T) {
	type testCase struct {
		In  []xml.Attr
		Out []xml.Attr
	}

	testCases := []testCase{
		testCase{
			In: []xml.Attr{
				xml.Attr{
					Name:  xml.Name{Space: "", Local: "xmlns"},
					Value: "https://example.com",
				},
				xml.Attr{
					Name:  xml.Name{Space: "foo", Local: "bar"},
					Value: "baz",
				},
			},
			Out: []xml.Attr{
				xml.Attr{
					Name:  xml.Name{Space: "", Local: "xmlns"},
					Value: "https://example.com",
				},
				xml.Attr{
					Name:  xml.Name{Space: "foo", Local: "bar"},
					Value: "baz",
				},
			},
		},
		testCase{
			In: []xml.Attr{
				xml.Attr{
					Name:  xml.Name{Space: "foo", Local: "bar"},
					Value: "baz",
				},
				xml.Attr{
					Name:  xml.Name{Space: "", Local: "xmlns"},
					Value: "https://example.com",
				},
			},
			Out: []xml.Attr{
				xml.Attr{
					Name:  xml.Name{Space: "", Local: "xmlns"},
					Value: "https://example.com",
				},
				xml.Attr{
					Name:  xml.Name{Space: "foo", Local: "bar"},
					Value: "baz",
				},
			},
		},
		testCase{
			In: []xml.Attr{
				xml.Attr{
					Name:  xml.Name{Space: "xmlns", Local: "foo"},
					Value: "https://example.com",
				},
				xml.Attr{
					Name:  xml.Name{Space: "foo", Local: "bar"},
					Value: "baz",
				},
			},
			Out: []xml.Attr{
				xml.Attr{
					Name:  xml.Name{Space: "xmlns", Local: "foo"},
					Value: "https://example.com",
				},
				xml.Attr{
					Name:  xml.Name{Space: "foo", Local: "bar"},
					Value: "baz",
				},
			},
		},
		testCase{
			In: []xml.Attr{
				xml.Attr{
					Name:  xml.Name{Space: "foo", Local: "bar"},
					Value: "baz",
				},
				xml.Attr{
					Name:  xml.Name{Space: "xmlns", Local: "foo"},
					Value: "https://example.com",
				},
			},
			Out: []xml.Attr{
				xml.Attr{
					Name:  xml.Name{Space: "xmlns", Local: "foo"},
					Value: "https://example.com",
				},
				xml.Attr{
					Name:  xml.Name{Space: "foo", Local: "bar"},
					Value: "baz",
				},
			},
		},
		testCase{
			In: []xml.Attr{
				xml.Attr{
					Name:  xml.Name{Space: "xmlns", Local: "foo"},
					Value: "https://example.com",
				},
				xml.Attr{
					Name:  xml.Name{Space: "xmlns", Local: "bar"},
					Value: "https://example.com",
				},
			},
			Out: []xml.Attr{
				xml.Attr{
					Name:  xml.Name{Space: "xmlns", Local: "bar"},
					Value: "https://example.com",
				},
				xml.Attr{
					Name:  xml.Name{Space: "xmlns", Local: "foo"},
					Value: "https://example.com",
				},
			},
		},
		testCase{
			In: []xml.Attr{
				xml.Attr{
					Name:  xml.Name{Space: "a", Local: "attr"},
					Value: "out",
				},
				xml.Attr{
					Name:  xml.Name{Space: "b", Local: "attr"},
					Value: "sorted",
				},
				xml.Attr{
					Name:  xml.Name{Space: "", Local: "attr2"},
					Value: "all",
				},
				xml.Attr{
					Name:  xml.Name{Space: "", Local: "attr"},
					Value: "I'm",
				},
			},
			Out: []xml.Attr{
				xml.Attr{
					Name:  xml.Name{Space: "", Local: "attr"},
					Value: "I'm",
				},
				xml.Attr{
					Name:  xml.Name{Space: "", Local: "attr2"},
					Value: "all",
				},
				xml.Attr{
					Name:  xml.Name{Space: "b", Local: "attr"},
					Value: "sorted",
				},
				xml.Attr{
					Name:  xml.Name{Space: "a", Local: "attr"},
					Value: "out",
				},
			},
		},
	}

	var s stack.Stack
	s.Push(map[string]string{
		"":  "http://example.com",
		"a": "http://www.w3.org",
		"b": "http://www.ietf.org",
	})

	for i, tt := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			attrs := sortattr.SortAttr{Attrs: tt.In, Stack: &s}
			sort.Sort(attrs)
			assert.Equal(t, tt.Out, attrs.Attrs)
		})
	}
}

// <e5 a:attr="out" b:attr="sorted" attr2="all" attr="I'm"
// xmlns:b="http://www.ietf.org"
// xmlns:a="http://www.w3.org"
// xmlns="http://example.org"/>
