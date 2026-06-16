package dockerfile

import (
	"sort"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/shell"
)

type envGetter struct {
	envs map[string]string
	args map[string]string
}

func (e *envGetter) Get(key string) (string, bool) {
	if e.envs != nil {
		if v, ok := e.envs[key]; ok {
			return v, true
		}
	}
	if e.args != nil {
		if v, ok := e.args[key]; ok {
			return v, true
		}
	}
	return "", false
}

func (e *envGetter) Keys() []string {
	seen := make(map[string]struct{})
	keys := make([]string, 0, len(e.envs)+len(e.args))
	for k := range e.envs {
		keys = append(keys, k)
		seen[k] = struct{}{}
	}
	for k := range e.args {
		if _, ok := seen[k]; !ok {
			keys = append(keys, k)
		}
	}
	return keys
}

type SingleWordExpander instructions.SingleWordExpander

// Use the dockerfile shell lexer to process commands that support variable expansion.
//
// This has the following effects:
// - Interpret single quotes, double quotes and escape sequences
// - Expand ENV and ARG variable references
//
// When injecting additional environment variables, InjectEnv()
// must be called first in order for Expand() to work properly.
func (d *Dockerfile) Expand(argExp SingleWordExpander) {
	d.expand(instructions.SingleWordExpander(argExp))
	d.analyzeStages()
}

func (d *Dockerfile) expand(argExp instructions.SingleWordExpander) {
	lex := shell.NewLex(d.escapeToken)

	metaArgs := d.buildMetaArgs(argExp)
	metaEnv := &envGetter{envs: metaArgs}

	for i, stage := range d.Stages {
		// ProcessWord errors on malformed shell syntax (unmatched quotes, bad substitutions).
		// The Expand() API doesn't return errors; just keep the unexpanded value on error.
		if expanded, _, err := lex.ProcessWord(d.Stages[i].BaseName, metaEnv); err == nil {
			d.Stages[i].BaseName = expanded
		}
		if expanded, _, err := lex.ProcessWord(d.Stages[i].Platform, metaEnv); err == nil {
			d.Stages[i].Platform = expanded
		}

		localArgs := make(map[string]string)
		localEnvs := make(map[string]string)

		localEnv := &envGetter{envs: localEnvs, args: localArgs}
		expandLocalVars := func(s string) (string, error) {
			expanded, _, err := lex.ProcessWord(s, localEnv)
			return expanded, err
		}

		for i := range stage.Commands {
			cmdExpander, ok := stage.Commands[i].Command.(instructions.SupportsSingleWordExpansion)
			if ok {
				// Keep the unexpanded value on errors, same rationale as above
				_ = cmdExpander.Expand(expandLocalVars)
			}

			// *After* expanding, update local variables
			switch command := stage.Commands[i].Command.(type) {
			case *instructions.EnvCommand:
				for _, env := range command.Env {
					localEnvs[env.Key] = env.Value
				}
			case *instructions.ArgCommand:
				for i := range command.Args {
					arg := &command.Args[i]
					if val, err := argExp(arg.Key); err == nil {
						// Override with externally supplied arg value
						arg.Value = &val
					} else if arg.Value == nil {
						// No default value and no external value - inherit from meta arg
						if metaVal, ok := metaArgs[arg.Key]; ok {
							arg.Value = &metaVal
						}
					}
					if arg.Value != nil {
						localArgs[arg.Key] = *arg.Value
					}
				}
			}
		}
	}
}

func (d *Dockerfile) buildMetaArgs(argExp instructions.SingleWordExpander) map[string]string {
	lex := shell.NewLex(d.escapeToken)

	metaArgs := make(map[string]string, len(d.MetaArgs))
	metaEnv := &envGetter{args: metaArgs}

	for _, arg := range d.MetaArgs {
		if val, err := argExp(arg.Key); err == nil {
			arg.ProvidedValue = &val
			// CLI-provided values are plain strings, don't expand them.
			arg.Value = &val
		} else if arg.Value != nil {
			// Keep the unexpanded value on errors, same rationale as in expand()
			if exp, _, err := lex.ProcessWord(*arg.Value, metaEnv); err == nil {
				arg.Value = &exp
			}
		}
		if arg.Value != nil {
			metaArgs[arg.Key] = *arg.Value
		}
	}

	return metaArgs
}

func (d *Dockerfile) InjectEnv(envs map[string]string) {
	if len(envs) == 0 {
		return
	}

	keysToInject := make([]string, 0, len(envs))
	for k := range envs {
		keysToInject = append(keysToInject, k)
	}
	sort.Strings(keysToInject)

	kvps := make([]instructions.KeyValuePair, 0, len(keysToInject))
	for _, k := range keysToInject {
		kvps = append(kvps, instructions.KeyValuePair{
			Key:     k,
			Value:   envs[k],
			NoDelim: false,
		})
	}

	for i := range d.Stages {
		cmd := Command{
			Command: &instructions.EnvCommand{
				Env: kvps,
			},
			Name: "ENV",
		}
		d.Stages[i].Commands = append([]*Command{&cmd}, d.Stages[i].Commands...)
	}
}
