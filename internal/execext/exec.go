package execext

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/moreinterp/coreutils"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"

	"github.com/go-task/task/v3/errors"
)

// ErrNilOptions is returned when a nil options is given
var ErrNilOptions = errors.New("execext: nil options given")

// RunCommandOptions is the options for the [RunCommand] func.
type RunCommandOptions struct {
	Command   string
	Dir       string
	Env       []string
	PosixOpts []string
	BashOpts  []string
	Sh        []string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
}

// RunCommand runs a shell command
func RunCommand(ctx context.Context, opts *RunCommandOptions) error {
	if opts == nil {
		return ErrNilOptions
	}

	// Set "-e" or "errexit" by default
	opts.PosixOpts = append(opts.PosixOpts, "e")

	// Format POSIX options into a slice that mvdan/sh understands
	var params []string
	for _, opt := range opts.PosixOpts {
		if len(opt) == 1 {
			params = append(params, fmt.Sprintf("-%s", opt))
		} else {
			params = append(params, "-o")
			params = append(params, opt)
		}
	}

	environ := opts.Env
	if len(environ) == 0 {
		environ = os.Environ()
	}

	r, err := interp.New(
		interp.Params(params...),
		interp.Env(expand.ListEnviron(environ...)),
		interp.ExecHandlers(execHandlers(opts.Sh)...),
		interp.OpenHandler(openHandler),
		interp.StdIO(opts.Stdin, opts.Stdout, opts.Stderr),
		dirOption(opts.Dir),
	)
	if err != nil {
		return err
	}

	parser := syntax.NewParser()

	// Run any shopt commands
	if len(opts.BashOpts) > 0 {
		shoptCmdStr := fmt.Sprintf("shopt -s %s", strings.Join(opts.BashOpts, " "))
		shoptCmd, err := parser.Parse(strings.NewReader(shoptCmdStr), "")
		if err != nil {
			return err
		}
		if err := r.Run(ctx, shoptCmd); err != nil {
			return err
		}
	}

	// Run the user-defined command
	p, err := parser.Parse(strings.NewReader(opts.Command), "")
	if err != nil {
		return err
	}
	return r.Run(ctx, p)
}

func escape(s string) string {
	s = filepath.ToSlash(s)
	s = strings.ReplaceAll(s, " ", `\ `)
	s = strings.ReplaceAll(s, "&", `\&`)
	s = strings.ReplaceAll(s, "(", `\(`)
	s = strings.ReplaceAll(s, ")", `\)`)
	return s
}

// ExpandLiteral is a wrapper around [expand.Literal]. It will escape the input
// string, expand any shell symbols (such as '~') and resolve any environment
// variables.
func ExpandLiteral(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	p := syntax.NewParser()
	word, err := p.Document(strings.NewReader(s))
	if err != nil {
		return "", err
	}
	cfg := &expand.Config{
		Env:      expand.FuncEnviron(os.Getenv),
		ReadDir2: os.ReadDir,
		GlobStar: true,
	}
	return expand.Literal(cfg, word)
}

// ExpandFields is a wrapper around [expand.Fields]. It will escape the input
// string, expand any shell symbols (such as '~') and resolve any environment
// variables. It also expands brace expressions ({a.b}) and globs (*/**) and
// returns the results as a list of strings.
func ExpandFields(s string) ([]string, error) {
	s = escape(s)
	p := syntax.NewParser()
	var words []*syntax.Word
	for w := range p.WordsSeq(strings.NewReader(s)) {
		words = append(words, w)
	}
	cfg := &expand.Config{
		Env:      expand.FuncEnviron(os.Getenv),
		ReadDir2: os.ReadDir,
		GlobStar: true,
		NullGlob: true,
	}
	return expand.Fields(cfg, words...)
}

// knownCommandStringShells maps the lowercase basename of well-known shells
// (without path and without the .exe extension on Windows) to the flag they
// use to accept a command string. When Task detects that the first element of
// a sh spec matches one of these shells and the last element matches its
// command-string flag, it activates join mode: all expanded args are
// POSIX-single-quote-joined into one string instead of being passed as
// separate OS-level arguments.
//
// To add support for a new shell, add an entry here.
var knownCommandStringShells = map[string]string{
	// POSIX-compatible shells
	"sh":   "-c",
	"bash": "-c",
	"zsh":  "-c",
	"dash": "-c",
	"ksh":  "-c",
	"fish": "-c",
	// PowerShell (cross-platform)
	"pwsh":       "-c",
	"powershell": "-c",
}

// isJoinMode reports whether sh should use join mode, i.e. whether all
// expanded args should be shell-quoted and concatenated into a single string
// that is passed as one OS argument to the shell's command-string flag.
//
// Join mode is activated when both conditions hold:
//  1. The basename of sh[0] (lowercased, .exe stripped) is a known
//     command-string shell.
//  2. The last element of sh equals that shell's command-string flag.
//
// This means "bash -c" activates join mode, but "bash -x" does not, and
// "myshell -c" does not (unless myshell is added to knownCommandStringShells).
func isJoinMode(sh []string) bool {
	if len(sh) < 2 {
		return false
	}
	// Extract the basename, strip path separators and the .exe suffix.
	base := filepath.Base(sh[0])
	base = strings.TrimSuffix(base, ".exe")
	base = strings.ToLower(base)

	flag, known := knownCommandStringShells[base]
	if !known {
		return false
	}
	return sh[len(sh)-1] == flag
}

func execHandlers(sh []string) (handlers []func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc) {
	if len(sh) > 0 {
		handlers = append(handlers, customShHandler(sh))
	}
	if useGoCoreUtils {
		handlers = append(handlers, coreutils.ExecHandler)
	}
	return handlers
}

// customShHandler returns an exec handler middleware that forwards command
// execution to the given custom shell. mvdan.cc/sh performs all shell
// processing (variable expansion, control flow, etc.) and calls this handler
// with the expanded command name and arguments. The handler then runs those
// args through the custom shell instead of directly.
//
// Two calling conventions are selected automatically:
//
//   - Separate-args mode: each expanded arg is appended as a separate OS
//     argument. This is the default and is suitable for prefix wrappers like
//     "docker run --rm alpine", "ssh user@host", "sudo".
//
//   - Join mode: all expanded args are POSIX-single-quote-escaped and joined
//     into one string that is passed as a single OS argument. Activated when
//     the shell is in [knownCommandStringShells] and the last sh element
//     matches its command-string flag. This makes "bash -c", "sh -c",
//     "zsh -c", "fish -c", "pwsh -c", etc. work correctly.
//     Note: POSIX single-quote escaping is compatible with POSIX shells but
//     PowerShell uses different quoting rules; commands without embedded
//     single quotes work fine with pwsh/powershell.
func customShHandler(sh []string) func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	joinMode := isJoinMode(sh)

	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			hc := interp.HandlerCtx(ctx)

			// Collect exported environment variables from the interpreter state.
			// This captures any variables set/exported by the shell script so far.
			var envList []string
			hc.Env.Each(func(name string, vr expand.Variable) bool {
				if vr.Exported && vr.Kind == expand.String {
					envList = append(envList, name+"="+vr.Str)
				}
				return true
			})

			// Build the argument list for the custom shell.
			var extraArgs []string
			if joinMode {
				// Join mode: pack all expanded args into a single shell-quoted
				// string so that the target shell (-c) re-parses them correctly.
				extraArgs = []string{shellJoin(args)}
			} else {
				extraArgs = args
			}

			cmd := exec.CommandContext(ctx, sh[0], append(sh[1:], extraArgs...)...)
			cmd.Dir = hc.Dir
			cmd.Env = envList
			cmd.Stdin = hc.Stdin
			cmd.Stdout = hc.Stdout
			cmd.Stderr = hc.Stderr

			if err := cmd.Run(); err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					return interp.ExitStatus(uint8(exitErr.ExitCode()))
				}
				return err
			}
			return nil
		}
	}
}

// shellJoin joins args into a single POSIX shell-quoted string suitable for
// passing as the command-string argument to a shell's -c flag.
// Each argument is individually single-quote escaped so that whitespace,
// metacharacters, and variable references within an argument are preserved
// exactly when the target shell re-parses the string.
// POSIX single-quote escaping is compatible with bash, sh, zsh, dash, etc.
// PowerShell uses different quoting rules but handles simple strings without
// embedded single quotes correctly.
func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shellQuote(arg)
	}
	return strings.Join(quoted, " ")
}

// shellQuote wraps s in POSIX single quotes, escaping any embedded single
// quotes using the standard `'\''` idiom. The result is safe to embed in a
// shell command string passed via -c.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func openHandler(ctx context.Context, path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	if path == "/dev/null" {
		return devNull{}, nil
	}
	return interp.DefaultOpenHandler()(ctx, path, flag, perm)
}

func dirOption(path string) interp.RunnerOption {
	return func(r *interp.Runner) error {
		err := interp.Dir(path)(r)
		if err == nil {
			return nil
		}

		// If the specified directory doesn't exist, it will be created later.
		// Therefore, even if `interp.Dir` method returns an error, the
		// directory path should be set only when the directory cannot be found.
		if absPath, _ := filepath.Abs(path); absPath != "" {
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				r.Dir = absPath
				return nil
			}
		}

		return err
	}
}
