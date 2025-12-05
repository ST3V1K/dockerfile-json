package dockerfile

import (
	"os"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
)

type SingleWordExpander instructions.SingleWordExpander

func (d *Dockerfile) Expand(argExp, envExp SingleWordExpander) {
	d.expand(instructions.SingleWordExpander(argExp), instructions.SingleWordExpander(envExp))
	d.analyzeStages()
}

func (d *Dockerfile) expand(argExp, envExp instructions.SingleWordExpander) {
	// Should be created only from the build args, no env variables here
	metaArgs := d.buildMetaArgs(argExp)
	for i, stage := range d.Stages {
		d.Stages[i].BaseName = os.Expand(d.Stages[i].BaseName, func(varname string) string {
			return metaArgs[varname]
		})

		localArgs := make(map[string]string)
		localEnvs := make(map[string]string)

		expandLocalVars := func(s string) (string, error) {
			expanded := os.Expand(s, func(varname string) string {
				// Containerfile defined ENVs take precedence over ENV variables defined through CLI (--env) and ARGs
				if envVal, ok := localEnvs[varname]; ok {
					return envVal
				}
				// Env variables provided from CLI
				if cliEnvVal, err := envExp(varname); err == nil {
					return cliEnvVal
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
