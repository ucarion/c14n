package c14n

import (
	"encoding/xml"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSortAttr exercizes a specific branch of sortAttr that is otherwise never
// tested: a non-empty namespace node appearing before an empty one.
//
// In practice, black-box testing cannot easily test that branch of logic,
// because writeStartElement in c14n.go will handle namespace nodes separately,
// and write them to the end.
//
// In principle, it might be possible to trigger this via end-to-end testing,
// but such a test would be tightly coupled with the details of what swaps
// sort.Sort will run. Rather than write what would be a rather inscrutable
// test, we simply test internals here directly.
//
// This test is coupled, in terms of test coverage, only to the fact that
// sort.Sort will make its initial call to Less with arguments i=1, j=0.
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
	}

	for i, tt := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			attrs := sortAttr{attrs: tt.In}
			sort.Sort(attrs)
			assert.Equal(t, tt.Out, attrs.attrs)
		})
	}
}
