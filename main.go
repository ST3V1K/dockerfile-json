package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"

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
	flag.Var(&config.EnvVars, "env", config.EnvVars.Help())

}

func parseFlags() {
	flag.Parse()

	if config.Quiet {
		log.SetOutput(ioutil.Discard)
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

func buildArgExpander() dockerfile.SingleWordExpander {
	args := make(map[string]string, len(config.BuildArgs.Values))

	// Detect user's os and arch
	buildOS := runtime.GOOS
	buildArch := runtime.GOARCH
	buildPlatform := buildOS + "/" + buildArch

	// Define built-in Docker ARG variables
	// See https://docs.docker.com/build/building/multi-platform/
	builtinArgs := map[string]string{
		"TARGETPLATFORM": buildPlatform,
		"TARGETARCH":     buildArch,
		"TARGETOS":       buildOS,
		"BUILDPLATFORM":  buildPlatform,
		"BUILDARCH":      buildArch,
		"BUILDOS":        buildOS,
	}

	// Add the builtin args to the environment
	for key, value := range builtinArgs {
		args[key] = value
	}

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
	}
}

func envExpander() dockerfile.SingleWordExpander {
	env := make(map[string]string, len(config.EnvVars.Values))

	for key, value := range config.EnvVars.Values {
		if value != nil {
			env[key] = *value
			continue
		}
		if value, ok := os.LookupEnv(key); ok {
			env[key] = value
		}
	}

	return func(word string) (string, error) {
		if value, ok := env[word]; ok {
			return value, nil
		}
		return "", fmt.Errorf("not defined: $%s", word)
	}
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
	if config.Expand {
		argExp := buildArgExpander()
		envExp := envExpander()
		for _, dockerfile := range dockerfiles {
			dockerfile.Expand(argExp, envExp)
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
