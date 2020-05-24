package c14n_test

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/ucarion/c14n"
	"golang.org/x/net/html/charset"
)

func TestCanonicalize(t *testing.T) {
	entries, err := ioutil.ReadDir("tests")
	assert.NoError(t, err)

	for _, file := range entries {
		t.Run(file.Name(), func(t *testing.T) {
			in, err := ioutil.ReadFile(fmt.Sprintf("tests/%s/in.xml", file.Name()))
			assert.NoError(t, err)

			out, err := ioutil.ReadFile(fmt.Sprintf("tests/%s/out.xml", file.Name()))
			assert.NoError(t, err)

			decoder := xml.NewDecoder(bytes.NewReader(in))
			decoder.CharsetReader = charset.NewReaderLabel

			actual, err := c14n.Canonicalize(decoder)
			assert.NoError(t, err)
			assert.Equal(t, out, actual)
		})
	}
}

func TestCanonicalize_NoStartElement(t *testing.T) {
	decoder := xml.NewDecoder(strings.NewReader("<!-- foo -->"))
	_, err := c14n.Canonicalize(decoder)
	assert.Equal(t, io.EOF, err)
}
