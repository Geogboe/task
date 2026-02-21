package ast

import (
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/go-task/task/v3/errors"
)

// ShArgs represents the shell/interpreter used to execute commands.
// When set, commands are run by appending the expanded command name and
// arguments as separate items to the specified shell. For example, setting
// sh to ["docker", "run", "--rm", "alpine"] will run commands as:
//
//	docker run --rm alpine <cmd> [args...]
//
// mvdan.cc/sh handles all shell processing (variable expansion, control flow,
// etc.) and calls the custom shell only for external (non-builtin) command
// execution, with the already-expanded command and arguments.
//
// This is well-suited for prefix-style wrappers such as:
//
//	sh: docker run --rm alpine
//	sh: ssh user@host
//	sh: sudo
//	sh: timeout 10
//
// Note: shells that expect a single command string argument (e.g. bash -c or
// sh -c) do not work with this model, because each argument is passed
// separately rather than as one string.
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
