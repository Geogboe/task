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
// Two calling conventions are supported automatically based on the last
// element of the sh specification:
//
// Separate-args mode (last element does NOT start with '-'):
//
//	Each expanded argument is passed as a separate OS-level argument.
//	Suitable for prefix wrappers:
//
//	  sh: docker run --rm alpine   →  docker run --rm alpine <cmd> [args...]
//	  sh: ssh user@host            →  ssh user@host <cmd> [args...]
//	  sh: sudo                     →  sudo <cmd> [args...]
//
// Join mode (last element starts with '-', e.g. -c):
//
//	All expanded arguments are shell-quoted and joined into a single string
//	that is passed as one OS-level argument. Suitable for shells that accept
//	a command string:
//
//	  sh: bash -c       →  bash -c '<cmd> [args...]'
//	  sh: sh -c         →  sh -c '<cmd> [args...]'
//	  sh: pwsh -nop -c  →  pwsh -nop -c '<cmd> [args...]'
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
