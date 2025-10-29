package dockerfile

import (
	"fmt"
	"os"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
)

type SingleWordExpander instructions.SingleWordExpander

func (d *Dockerfile) Expand(env SingleWordExpander) {
	d.expand(instructions.SingleWordExpander(env))
	d.analyzeStages()
}

func (d *Dockerfile) expand(env instructions.SingleWordExpander) {
	metaArgsEnvExpander := d.metaArgsEnvExpander(env)

	// Create an os.Expand-compatible function from the SingleWordExpander
	expandFunc := func(key string) string {
		value, err := metaArgsEnvExpander(key)
		if err != nil {
			return ""
		}
		return value
	}

	for i, stage := range d.Stages {
		d.Stages[i].BaseName = os.Expand(stage.BaseName, expandFunc)

		for i := range stage.Commands {
			// Expand commands using os.Expand instead of delegating to buildkit
			// This allows complex expressions like "$VERSION+git-$REVISION" to work
			d.expandCommand(stage.Commands[i], expandFunc)
		}
	}
}

// expandCommand expands variables in command values using os.Expand
func (d *Dockerfile) expandCommand(cmd *Command, expandFunc func(string) string) {
	switch c := cmd.Command.(type) {
	case *instructions.LabelCommand:
		// Expand LABEL key-value pairs
		for i := range c.Labels {
			c.Labels[i].Key = os.Expand(c.Labels[i].Key, expandFunc)
			c.Labels[i].Value = os.Expand(c.Labels[i].Value, expandFunc)
		}
	case *instructions.EnvCommand:
		// Expand ENV key-value pairs
		for i := range c.Env {
			c.Env[i].Key = os.Expand(c.Env[i].Key, expandFunc)
			c.Env[i].Value = os.Expand(c.Env[i].Value, expandFunc)
		}
	case *instructions.ArgCommand:
		// Expand ARG values
		for i := range c.Args {
			if c.Args[i].Value != nil {
				expanded := os.Expand(*c.Args[i].Value, expandFunc)
				c.Args[i].Value = &expanded
			}
		}
	case *instructions.WorkdirCommand:
		// Expand WORKDIR path
		c.Path = os.Expand(c.Path, expandFunc)
	case *instructions.UserCommand:
		// Expand USER value
		c.User = os.Expand(c.User, expandFunc)
	case instructions.SupportsSingleWordExpansion:
		// For other commands that support expansion, use buildkit's method
		// This is a fallback for commands we haven't explicitly handled
		c.Expand(func(word string) (string, error) {
			return os.Expand(word, expandFunc), nil
		})
	}
}

func (d *Dockerfile) metaArgsEnvExpander(env instructions.SingleWordExpander) instructions.SingleWordExpander {
	metaArgsEnv := make(map[string]string, len(d.MetaArgs))
	for _, arg := range d.MetaArgs {
		if defaultValue := arg.DefaultValue; defaultValue != nil {
			metaArgsEnv[arg.Key] = *defaultValue
		}

		if value, err := env(arg.Key); err == nil {
			arg.ProvidedValue = &value
			metaArgsEnv[arg.Key] = value
			arg.Value = &value
		}

		exp := os.Expand(*arg.Value, func(argval string) string {
			if val, ok := metaArgsEnv[argval]; ok {
				return val;
			}

			return argval;
		})
		arg.Value = &exp;
		metaArgsEnv[arg.Key] = exp;
	}

	return func(key string) (string, error) {
		if value, ok := metaArgsEnv[key]; ok {
			return value, nil
		}
		return "", fmt.Errorf("not defined: $%s", key)
	}
}
