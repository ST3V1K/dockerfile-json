package dockerfile

import (
	"os"
	"sort"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
)

type SingleWordExpander instructions.SingleWordExpander

// Expand the ENV and ARG variable references in instructions.
//
// When injecting additional environment variables, InjectEnv()
// must be called first in order for Expand() to work properly.
func (d *Dockerfile) Expand(argExp SingleWordExpander) {
	d.expand(instructions.SingleWordExpander(argExp))
	d.analyzeStages()
}

func (d *Dockerfile) expand(argExp instructions.SingleWordExpander) {
	metaArgs := d.buildMetaArgs(argExp)
	for i, stage := range d.Stages {
		d.Stages[i].BaseName = os.Expand(d.Stages[i].BaseName, func(varname string) string {
			return metaArgs[varname]
		})

		localArgs := make(map[string]string)
		localEnvs := make(map[string]string)

		expandLocalVars := func(s string) (string, error) {
			expanded := os.Expand(s, func(varname string) string {
				// ENVs take precedence over ARGs
				if envVal, ok := localEnvs[varname]; ok {
					return envVal
				}
				if argVal, ok := localArgs[varname]; ok {
					return argVal
				}
				return ""
			})
			return expanded, nil
		}

		for i := range stage.Commands {
			cmdExpander, ok := stage.Commands[i].Command.(instructions.SupportsSingleWordExpansion)
			if ok {
				cmdExpander.Expand(expandLocalVars)
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
	metaArgs := make(map[string]string, len(d.MetaArgs))
	for _, arg := range d.MetaArgs {
		if defaultValue := arg.DefaultValue; defaultValue != nil {
			metaArgs[arg.Key] = *defaultValue
		}

		if value, err := argExp(arg.Key); err == nil {
			arg.ProvidedValue = &value
			metaArgs[arg.Key] = value
			arg.Value = &value
		}

		if arg.Value != nil {
			exp := os.Expand(*arg.Value, func(varname string) string {
				return metaArgs[varname]
			})
			arg.Value = &exp
			metaArgs[arg.Key] = exp
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
