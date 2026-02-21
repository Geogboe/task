package execext

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShellQuote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{"hello", "'hello'"},
		{"hello world", "'hello world'"},
		{"it's here", "'it'\\''s here'"},
		{"", "''"},
		{"$VAR", "'$VAR'"},
		{"a&b|c", "'a&b|c'"},
	}

	for _, tc := range tests {
		got := shellQuote(tc.in)
		assert.Equal(t, tc.want, got, "shellQuote(%q)", tc.in)
	}
}

func TestShellJoin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args []string
		want string
	}{
		{[]string{"echo", "hello"}, "'echo' 'hello'"},
		{[]string{"echo", "hello world"}, "'echo' 'hello world'"},
		{[]string{"env", "echo", "it's fine"}, "'env' 'echo' 'it'\\''s fine'"},
		{[]string{"cmd"}, "'cmd'"},
	}

	for _, tc := range tests {
		got := shellJoin(tc.args)
		assert.Equal(t, tc.want, got, "shellJoin(%v)", tc.args)
	}
}
