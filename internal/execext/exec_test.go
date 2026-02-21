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

func TestIsJoinMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sh   []string
		want bool
		desc string
	}{
		// Known shells with correct -c flag → join mode
		{[]string{"bash", "-c"}, true, "bash -c"},
		{[]string{"sh", "-c"}, true, "sh -c"},
		{[]string{"zsh", "-c"}, true, "zsh -c"},
		{[]string{"dash", "-c"}, true, "dash -c"},
		{[]string{"ksh", "-c"}, true, "ksh -c"},
		{[]string{"fish", "-c"}, true, "fish -c"},
		{[]string{"pwsh", "-c"}, true, "pwsh -c"},
		{[]string{"powershell", "-c"}, true, "powershell -c"},
		// Extra flags before -c still activate join mode (flag is last)
		{[]string{"pwsh", "-nop", "-c"}, true, "pwsh -nop -c"},
		{[]string{"bash", "-x", "-c"}, true, "bash -x -c"},
		// Full paths: basename extraction
		{[]string{"/bin/bash", "-c"}, true, "/bin/bash -c"},
		{[]string{"/usr/bin/zsh", "-c"}, true, "/usr/bin/zsh -c"},
		// Windows .exe suffix stripped
		{[]string{"pwsh.exe", "-c"}, true, "pwsh.exe -c"},
		{[]string{"bash.exe", "-c"}, true, "bash.exe -c"},
		// Known shell but wrong flag → separate-args
		{[]string{"bash", "-x"}, false, "bash -x (wrong flag)"},
		{[]string{"bash"}, false, "bash alone (no flag)"},
		// Known shell but flag is not last → separate-args
		{[]string{"bash", "-c", "somearg"}, false, "bash -c with trailing arg"},
		// Unknown shell with -c → separate-args (not in known list)
		{[]string{"myshell", "-c"}, false, "unknown shell with -c"},
		{[]string{"docker", "-c"}, false, "docker with -c"},
		// Prefix wrappers → separate-args
		{[]string{"docker", "run", "--rm", "alpine"}, false, "docker run"},
		{[]string{"ssh", "user@host"}, false, "ssh prefix"},
		{[]string{"sudo"}, false, "sudo alone"},
		// Empty
		{[]string{}, false, "empty"},
		{[]string{"bash"}, false, "single element"},
	}

	for _, tc := range tests {
		got := isJoinMode(tc.sh)
		assert.Equal(t, tc.want, got, tc.desc)
	}
}
