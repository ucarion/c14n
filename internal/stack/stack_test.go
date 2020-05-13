package stack_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/ucarion/c14n/internal/stack"
)

func TestStack(t *testing.T) {
	var s stack.Stack

	assert.Equal(t, 0, s.Len())
	assert.Equal(t, "", s.Get("unknown"))
	assert.Equal(t, map[string]string(nil), s.Peek())
	assert.Equal(t, map[string]string{}, s.Used())

	s.Push(map[string]string{
		"foo": "http://example.com/foo",
		"bar": "http://example.com/bar",
	})

	assert.Equal(t, 1, s.Len())
	assert.Equal(t, "http://example.com/foo", s.Get("foo"))
	assert.Equal(t, map[string]string{"foo": "http://example.com/foo", "bar": "http://example.com/bar"}, s.Peek())
	assert.Equal(t, map[string]string{"foo": "http://example.com/foo"}, s.Used())

	s.Push(map[string]string{
		"foo": "http://example.com/foo/new",
	})

	assert.Equal(t, 2, s.Len())
	assert.Equal(t, "http://example.com/foo/new", s.Get("foo"))
	assert.Equal(t, map[string]string{"foo": "http://example.com/foo/new"}, s.Peek())
	assert.Equal(t, map[string]string{"foo": "http://example.com/foo/new"}, s.Used())

	s.Pop()

	assert.Equal(t, 1, s.Len())
	assert.Equal(t, "http://example.com/foo", s.Get("foo"))
	assert.Equal(t, map[string]string{"foo": "http://example.com/foo", "bar": "http://example.com/bar"}, s.Peek())
	assert.Equal(t, map[string]string{"foo": "http://example.com/foo"}, s.Used())

	s.Pop()

	assert.Equal(t, 0, s.Len())
	assert.Equal(t, map[string]string(nil), s.Peek())
	assert.Equal(t, map[string]string{}, s.Used())
}
