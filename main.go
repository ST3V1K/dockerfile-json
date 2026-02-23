package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/containerd/platforms"
	"github.com/keilerkonzept/dockerfile-json/pkg/buildargs"
	"github.com/keilerkonzept/dockerfile-json/pkg/dockerfile"
	"github.com/yalp/jsonpath"
)

var config struct {
	Quiet          bool
	Expand         bool
	JSONPathString string
	JSONPath       jsonpath.FilterFunc
	JSONPathRaw    bool
	BuildArgs      AssignmentsMap
	BuildArgFile   string
	EnvVars        AssignmentsMap
	NonzeroExit    bool
}

var name = "dockerfile-json"
var version = "dev"
var jsonOut = json.NewEncoder(os.Stdout)

func init() {
	log.SetOutput(os.Stderr)
	log.SetFlags(0)
	log.SetPrefix(fmt.Sprintf("[%s %s] ", filepath.Base(name), version))

	config.Expand = true
	flag.BoolVar(&config.Quiet, "quiet", config.Quiet, "suppress log output (stderr)")
	flag.BoolVar(&config.Expand, "expand-build-args", config.Expand, "DEPRECATED: use --expand-vars (expands ARG and ENV variables)")
	flag.BoolVar(&config.Expand, "expand-vars", config.Expand, "expand build ARGs and ENV variables")
	flag.StringVar(&config.JSONPathString, "jsonpath", config.JSONPathString, "select parts of the output using JSONPath (https://goessner.net/articles/JsonPath)")
	flag.BoolVar(&config.JSONPathRaw, "jsonpath-raw", config.JSONPathRaw, "when using JSONPath, output raw strings, not JSON values")
	flag.Var(&config.BuildArgs, "build-arg", config.BuildArgs.Help())
	flag.StringVar(&config.BuildArgFile, "build-arg-file", "",
		"path to a file containing build-args in the form argument=value (blank lines and # comments are ignored)")
	flag.Var(&config.EnvVars, "env", config.EnvVars.Help())

}

func parseFlags() {
	flag.Parse()

	if config.Quiet {
		log.SetOutput(io.Discard)
	}

	if flag.NArg() == 0 {
		flag.Usage()
	}

	if jsonPathString := config.JSONPathString; jsonPathString != "" {
		if jsonPathString[0] != '$' {
			jsonPathString = "$" + jsonPathString
		}
		jsonPath, err := jsonpath.Prepare(jsonPathString)
		if err != nil {
			log.Fatalf("parse jsonpath %s: %v", jsonPathString, err)
		}
		config.JSONPath = jsonPath
	}
}

func buildArgExpander() (dockerfile.SingleWordExpander, error) {
	args := make(map[string]string, len(config.BuildArgs.Values))

	platformSpec := platforms.DefaultSpec()
	buildPlatform := platforms.Format(platformSpec)

	// Define built-in Docker ARG variables
	// See https://docs.docker.com/build/building/multi-platform/
	builtinArgs := map[string]string{
		"BUILDPLATFORM":  buildPlatform,
		"BUILDOS":        platformSpec.OS,
		"BUILDARCH":      platformSpec.Architecture,
		"BUILDVARIANT":   platformSpec.Variant,
		"TARGETPLATFORM": buildPlatform,
		"TARGETOS":       platformSpec.OS,
		"TARGETARCH":     platformSpec.Architecture,
		"TARGETVARIANT":  platformSpec.Variant,
	}

	// Add the builtin args to the environment
	for key, value := range builtinArgs {
		args[key] = value
	}

	if config.BuildArgFile != "" {
		fileArgs, err := buildargs.ParseBuildArgFile(config.BuildArgFile)
		if err != nil {
			return nil, fmt.Errorf("parse build arg file %q: %w", config.BuildArgFile, err)
		}
		for key, value := range fileArgs {
			args[key] = value
		}
	}

	// Build args specified with --build-arg take precedence over build args
	// defined in build arg file
	for key, value := range config.BuildArgs.Values {
		if value != nil {
			args[key] = *value
			continue
		}
		if value, ok := os.LookupEnv(key); ok {
			args[key] = value
		}
	}

	return func(word string) (string, error) {
		if value, ok := args[word]; ok {
			return value, nil
		}
		return "", fmt.Errorf("not defined: $%s", word)
	}, nil
}

func main() {
	parseFlags()

	var dockerfiles []*dockerfile.Dockerfile
	for _, path := range flag.Args() {
		dockerfile, err := dockerfile.Parse(path)
		if err != nil {
			log.Printf("error: parse %q: %v", path, err)
			config.NonzeroExit = true
			continue
		}
		dockerfiles = append(dockerfiles, dockerfile)
	}

	if len(config.EnvVars.Values) > 0 {
		envsToInject := make(map[string]string)
		for key, value := range config.EnvVars.Values {
			if value != nil {
				envsToInject[key] = *value
			} else if value, ok := os.LookupEnv(key); ok {
				// If the value was not provided e.g --env FOO, then value is taken from host
				envsToInject[key] = value
			}
		}
		for _, dockerfile := range dockerfiles {
			dockerfile.InjectEnv(envsToInject)
		}
	}

	if config.Expand {
		argExp, err := buildArgExpander()
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		for _, dockerfile := range dockerfiles {
			dockerfile.Expand(argExp)
		}
	}
	switch {
	case config.JSONPath != nil:
		for _, dockerfile := range dockerfiles {
			rawJSON, err := json.Marshal(dockerfile)
			if err != nil {
				log.Printf("error: evaluate jsonpath: %v", err)
				config.NonzeroExit = true
				continue
			}
			var data map[string]interface{}
			if err := json.Unmarshal(rawJSON, &data); err != nil {
				log.Printf("error: evaluate jsonpath: %v", err)
				config.NonzeroExit = true
				continue
			}
			result, err := config.JSONPath(data)
			if err != nil {
				log.Printf("error: evaluate jsonpath: %v", err)
				config.NonzeroExit = true
				continue
			}
			values, isArray := result.([]interface{})
			value, isString := result.(string)
			switch {
			case isString && config.JSONPathRaw:
				fmt.Println(value)
			case isArray && config.JSONPathRaw:
				for _, value := range values {
					fmt.Println(value)
				}
			case isArray && !config.JSONPathRaw:
				for _, value := range values {
					jsonOut.Encode(value)
				}
			default:
				jsonOut.Encode(result)
			}

		}
	default:
		for _, dockerfile := range dockerfiles {
			jsonOut.Encode(dockerfile)
		}
	}
	if config.NonzeroExit {
		os.Exit(1)
	}
}
