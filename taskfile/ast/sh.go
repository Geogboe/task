package ast

import (
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/go-task/task/v3/errors"
)

// ShArgs represents the shell/interpreter used to execute commands.
// When set, commands are run by appending the expanded command name and
// arguments to the specified shell. mvdan.cc/sh handles all shell processing
// (variable expansion, control flow, etc.) and the custom shell is invoked
// only for external (non-builtin) command execution.
//
// Two calling conventions are selected automatically:
//
// Separate-args mode (default):
//
//	Each expanded argument is passed as a separate OS-level argument.
//	Suitable for prefix wrappers:
//
//	  sh: docker run --rm alpine   →  docker run --rm alpine <cmd> [args...]
//	  sh: ssh user@host            →  ssh user@host <cmd> [args...]
//	  sh: sudo                     →  sudo <cmd> [args...]
//
// Join mode (activated for known shells paired with their command-string flag):
//
//	All expanded arguments are POSIX-single-quote-escaped and joined into a
//	single string passed as one OS-level argument. Task recognizes the
//	following shells and their flags automatically:
//
//	  sh: bash -c       →  bash -c '<cmd> [args...]'
//	  sh: sh -c         →  sh -c '<cmd> [args...]'
//	  sh: zsh -c        →  zsh -c '<cmd> [args...]'
//	  sh: dash -c       →  dash -c '<cmd> [args...]'
//	  sh: ksh -c        →  ksh -c '<cmd> [args...]'
//	  sh: fish -c       →  fish -c '<cmd> [args...]'
//	  sh: pwsh -c       →  pwsh -c '<cmd> [args...]'
//	  sh: powershell -c →  powershell -c '<cmd> [args...]'
//	  sh: pwsh -nop -c  →  pwsh -nop -c '<cmd> [args...]'
//
//	Join mode requires both: a recognized shell name as the first element
//	and the shell's command-string flag as the last element.
//	"bash -x" or "myshell -c" use separate-args mode.
//
//	Note: POSIX single-quote escaping is used for all shells in join mode.
//	This works for POSIX-compatible shells (bash, sh, zsh, dash, ksh, fish)
//	but PowerShell uses different quoting rules. Commands that do not contain
//	embedded single quotes work correctly with pwsh/powershell.
//
// In YAML, this can be specified as a string (split on whitespace) or a list:
//
//	sh: docker run --rm alpine
//	sh: [docker, run, --rm, alpine]
//
// Warning: the string form is split on whitespace using strings.Fields, so
// arguments that contain spaces must use the list form instead.
type ShArgs []string

func (s *ShArgs) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		*s = strings.Fields(node.Value)
		return nil
	case yaml.SequenceNode:
		var args []string
		if err := node.Decode(&args); err != nil {
			return errors.NewTaskfileDecodeError(err, node)
		}
		*s = args
		return nil
	}
	return errors.NewTaskfileDecodeError(nil, node).WithTypeMessage("sh")
}
