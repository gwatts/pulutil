package policy

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

type mocks int

func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name + "_id", args.Inputs, nil
}

func (mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

func TestPolicy(t *testing.T) {
	assert := assert.New(t)

	expected := &Policy{
		Version: "2012-10-17",
		ID:      "test-id",
		Statement: Stmts{
			Stmt{
				Sid:    "stmt1",
				Effect: Deny,
				Principal: map[string]Strings{
					"AWS":           {"p1"},
					"CanonicalUser": {"u1", "u2"},
				},
				NotPrincipal: map[string]Strings{
					"AWS": {"not"},
				},
				Action: Strings{"action1", "action2"},
				Resource: Strings{"r1", "r2", "r3",
					pulumi.StringArray{
						pulumi.String("r4"), pulumi.String("r5")}},
				NotResource: Strings{"r4"},
			},
		},
	}

	p := New("test-id",
		Statement("stmt1",
			Effect(Deny),
			Action("action1", "action2"),
			Principal("AWS", "p1"),
			Principal("CanonicalUser", "u1", "u2"),
			NotPrincipal("AWS", "not"),
			Resource("r1"),
			Resource("r2", "r3"),
			Resource(pulumi.StringArray{pulumi.String("r4"), pulumi.String("r5")}),
			NotResource("r4"),
		),
	)

	assert.Equal(p, expected)
}

var stringsTests = []struct {
	name     string
	input    Strings
	expected string
}{
	{
		name:     "empty",
		input:    Strings{},
		expected: `[]`,
	}, {
		name:     "single",
		input:    Strings{"one"},
		expected: `"one"`,
	}, {
		name:     "two",
		input:    Strings{"one", "two"},
		expected: `["one","two"]`,
	},
}

func TestStringsJSON(t *testing.T) {
	for _, test := range stringsTests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)
			out, err := json.Marshal(test.input)
			assert.Nil(err)
			assert.Equal(test.expected, string(out))
		})
	}
}

func TestStringArrayJSON(t *testing.T) {
	assert := assert.New(t)

	var wg sync.WaitGroup
	wg.Add(1)
	_ = pulumi.RunErr(func(ctx *pulumi.Context) error {
		p := New("id",
			Statement("stmt1",
				Effect(Allow),
				Action("action"),
				Principal("AWS", "aws-id",
					pulumi.StringArray{pulumi.String("id2"), pulumi.String("id3")}),
				Resource(pulumi.StringArray{pulumi.String("r1")}),
			),
		)

		expected := `{
			"Id": "id", 
			"Version": "2012-10-17",
			"Statement": {
					"Sid": "stmt1",
					"Effect": "Allow",
					"Action": "action",
					"Principal": {"AWS": ["aws-id", "id2", "id3"]},
					"Resource": "r1"
			}
		}`
		p.ToStringOutput().ApplyT(func(js string) int {
			assert.JSONEq(expected, js)
			wg.Done()
			return 0
		})
		return nil
	}, pulumi.WithMocks("project", "stack", mocks(0)))
	wg.Wait()
}

func TestMultiStmtJSON(t *testing.T) {
	assert := assert.New(t)

	var wg sync.WaitGroup
	wg.Add(1)
	_ = pulumi.RunErr(func(ctx *pulumi.Context) error {
		p := New("id",
			Statement("stmt1",
				Effect(Allow),
				Principal("AWS", "aws-id"),
				Action("s3:GetObject"),
				Resource("arn1"),
			),
			Statement("stmt2",
				Effect(Deny),
				NotPrincipal("AWS", pulumi.String("aws-id2")),
				NotAction(pulumi.String("s3:PutObject")),
				NotResource(pulumi.String("arn2")),
			),
			Statement("stmt3",
				Effect(Allow),
				Principal("AWS", "aws-id"),
				Principal("Other", "id2", pulumi.String("id3")),
				Action("s3:GetObject", pulumi.String("s3:OtherOp")),
				Resource("arn1", pulumi.String("arn2")),
			),
		)

		expected := `{
			"Id": "id", 
			"Version": "2012-10-17",
			"Statement": [
				{
					"Sid": "stmt1",
					"Effect": "Allow",
					"Principal": {"AWS": "aws-id"},
					"Action": "s3:GetObject",
					"Resource": "arn1"
				}, {
					"Sid": "stmt2",
					"Effect": "Deny",
					"NotPrincipal": {"AWS": "aws-id2"},
					"NotAction": "s3:PutObject",
					"NotResource": "arn2"
				}, {
					"Sid": "stmt3",
					"Effect": "Allow",
					"Principal": {"AWS": "aws-id", "Other": ["id2", "id3"]},
					"Action": ["s3:GetObject", "s3:OtherOp"],
					"Resource": ["arn1", "arn2"]
				}
			]
		}`
		p.ToStringOutput().ApplyT(func(js string) int {
			assert.JSONEq(expected, js)
			wg.Done()
			return 0
		})
		return nil
	}, pulumi.WithMocks("project", "stack", mocks(0)))
	wg.Wait()
}
