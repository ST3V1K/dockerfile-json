package dockerfile

import (
	"os"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
)

type SingleWordExpander instructions.SingleWordExpander

func (d *Dockerfile) Expand(env SingleWordExpander) {
	d.expand(instructions.SingleWordExpander(env))
	d.analyzeStages()
}

func (d *Dockerfile) expand(env instructions.SingleWordExpander) {
	expandMetaArgs := d.metaArgsEnvExpander(env)
	for i, stage := range d.Stages {
		// Ignore error, expandMetaArgs never errors
		d.Stages[i].BaseName, _ = expandMetaArgs(d.Stages[i].BaseName)
		for i := range stage.Commands {
			cmdExpander, ok := stage.Commands[i].Command.(instructions.SupportsSingleWordExpansion)
			if ok {
				cmdExpander.Expand(expandMetaArgs)
			}
		}
	}
}

func (d *Dockerfile) metaArgsEnvExpander(env instructions.SingleWordExpander) instructions.SingleWordExpander {
	metaArgs := make(map[string]string, len(d.MetaArgs))
	expandMetaArgs := func(s string) string {
		return os.Expand(s, func(varname string) string {
			// Expand to metaArg value or empty string (if undefined)
			return metaArgs[varname]
		})
	}
	for _, arg := range d.MetaArgs {
		if defaultValue := arg.DefaultValue; defaultValue != nil {
			metaArgs[arg.Key] = *defaultValue
		}

		if value, err := env(arg.Key); err == nil {
			arg.ProvidedValue = &value
			metaArgs[arg.Key] = value
			arg.Value = &value
		}

		exp := expandMetaArgs(*arg.Value)
		arg.Value = &exp
		metaArgs[arg.Key] = exp
	}

	return func(key string) (string, error) {
		return expandMetaArgs(key), nil
	}
}
