// Package template defines helpers to make it easy to use Pulumi outputs
// as values with Go string templates.
//
// This essentially takes a number of outputs and wraps the template with a
// call to ApplyT to ensure the template is only exeucted once the supplied
// values have been resolved.
//
// See the example for NewJSON for an example of how to use this with Pulumi.
package template

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	tpl "text/template"

	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

var (
	// ErrCompileError is raised via panic if the template generates an
	// error during the compile process.
	ErrCompileError = errors.New("template compile error")

	// ErrExecuteError is raised during panic if template generates an error
	// while it is being executed (eg. due to an incorrectly supplied argument)
	ErrExecuteError = errors.New("template execution error")

	// ErrInvalidJSON is raised during panic if the output from the template
	// does not validate as JSON.
	ErrInvalidJSON = errors.New("template produced invalid JSON")
)

var (
	noPanic           bool
	m                 sync.Mutex
	testTemplateError error
)

func templateError(msg string, args ...interface{}) string {
	err := fmt.Errorf(msg, args...)
	if noPanic {
		m.Lock()
		testTemplateError = err
		m.Unlock()
	} else {
		panic(err)
	}
	return err.Error()
}

// New compiles a Go text/template and provides the specified variables
// to it, once they become available.
//
// This is analogous to pulumi.Sprintf, except that you can supply a template
// instead of a string.
//
// vars specifies a map of values to pass as data to the template; this may
// include any mix of regular values, or Pulumi outputs, which will have their
// values resolved before being supplied to the template.
func New(vars map[string]interface{}, templateText string) pulumi.StringOutput {
	return renderTemplate(vars, templateText, false)
}

// NewJSON wraps Template, but will panic if the rendered template does not
// parse as valid JSON.
func NewJSON(vars map[string]interface{}, templateText string) pulumi.StringOutput {
	return renderTemplate(vars, templateText, true)
}

func renderTemplate(vars map[string]interface{}, templateText string, validateJSON bool) pulumi.StringOutput {
	tpl, err := tpl.New("tpl").Parse(templateText)
	if err != nil {
		return pulumi.String(templateError("%w: %v", ErrCompileError, err)).ToStringOutput()

	}
	args := make([]interface{}, 0, len(vars))
	names := make([]string, 0, len(vars))
	for k, v := range vars {
		names = append(names, k)
		args = append(args, v)
	}

	return pulumi.All(args...).ApplyT(func(args []interface{}) string {
		finalVars := make(map[string]interface{})
		for i, v := range args {
			finalVars[names[i]] = v
		}
		var compiled strings.Builder
		if err := tpl.Execute(&compiled, finalVars); err != nil {
			return templateError("%w: %v", ErrExecuteError, err)
		}
		result := compiled.String()
		if validateJSON {
			var tmp interface{}
			if err := json.Unmarshal([]byte(result), &tmp); err != nil {
				if jerr, ok := err.(*json.SyntaxError); ok {
					return templateError("%w: Template does not compile to valid JSON with syntax error at byte %d: %v\n%s",
						ErrInvalidJSON, jerr.Offset, jerr, result)
				}
				return templateError("%w: Template does not compile to valid JSON: %v\n%s",
					ErrInvalidJSON, err, result)
			}
		}
		return result
	}).(pulumi.StringOutput)
}
