package template

import (
	"errors"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/tj/assert"
)

func init() {
	noPanic = true
}

type mocks int

func (mocks) NewResource(typeToken, name string, inputs resource.PropertyMap, provider, id string) (string, resource.PropertyMap, error) {
	return name + "_id", inputs, nil
}

func (mocks) Call(token string, args resource.PropertyMap, provider string) (resource.PropertyMap, error) {
	return args, nil
}

type tplTest struct {
	testName       string
	tplText        string
	asJSON         bool
	expectedError  error
	expectedResult string
}

func trap(err chan error, f func()) {
	defer func() {
		if v := recover(); v != nil {
			if perr, ok := v.(error); ok {
				err <- perr
			}
		}
	}()
	f()
}

func (tt *tplTest) run(t *testing.T) {
	testTemplateError = nil
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		var wg sync.WaitGroup
		var tpl pulumi.StringOutput
		if tt.asJSON {
			tpl = NewJSON(map[string]interface{}{
				"StringOut":    pulumi.String("ok!").ToStringOutput(),
				"NormalString": "normal",
			}, tt.tplText)
		} else {
			tpl = New(map[string]interface{}{
				"StringOut":    pulumi.String("ok!").ToStringOutput(),
				"NormalString": "normal",
			}, tt.tplText)
		}

		wg.Add(1)
		tpl.ApplyString(func(result string) string {
			defer wg.Done()
			if tt.expectedResult != "" {
				assert.Equal(t, tt.expectedResult, result, tt.testName)
			}
			return result
		})

		wg.Wait()
		return nil
	}, pulumi.WithMocks("project", "stack", mocks(0)))
	assert.NoError(t, err)

	if tt.expectedError == nil {
		if testTemplateError != nil {
			assert.Fail(t, "unexpected error", "[%s] Unexpected error: %v", tt.testName, testTemplateError)
		}
	} else if !errors.Is(testTemplateError, tt.expectedError) {
		assert.Fail(t, "incorrect error", "[%s] Expected error %q, got %q",
			tt.testName, tt.expectedError, testTemplateError)
	}
}

var tests = []tplTest{
	{
		testName:       "simple-ok",
		tplText:        `result: {{.StringOut}}\nline2: {{.NormalString}}`,
		expectedResult: `result: ok!\nline2: normal`,
	},
	{
		testName:      "invalid-template",
		tplText:       `result: {{.StringOut}`,
		expectedError: ErrCompileError,
	},
	{
		testName:      "invalid-reference",
		tplText:       `result: {{.StringOut.Foo}}`,
		expectedError: ErrExecuteError,
	},
	{
		testName:       "json-ok",
		tplText:        `{"field": "{{.StringOut}}"}`,
		expectedResult: `{"field": "ok!"}`,
	},
	{
		testName:      "invalid-json",
		asJSON:        true,
		tplText:       `result: {{.StringOut}}`,
		expectedError: ErrInvalidJSON,
	},
}

func TestTemplates(t *testing.T) {
	for _, test := range tests {
		test.run(t)
	}
}
