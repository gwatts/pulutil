// Package policy provides a helper type for generating JSON AWS policy
// documents.
//
// See the AWS documentation for policy elements used here:
// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements.html
//
// Policies can be created by passing configuration arguments to New that
// build statements, add resources, actions, effects, etc.
//
// Most elements accept strings, string slices, Pulumi StringOutputs or
// StringArrayOutputs.   Slices and arrays are flattened into a single list
// while building the final output which can save some work slicing them
// elsewhere.
//
//    bucketPolicy, err := s3.NewBucketPolicy(ctx, "bucket-policy", &s3.BucketPolicyArgs{
//        Bucket: newBucket.Bucket,
//        Policy: policy.New("my-bucket-policy",
//           policy.Statement("cross-account-access",
//               policy.Effect(policy.Allow),
//               policy.Action(
//                   "s3:GetObject",
//                   "s3:PutObject",
//               ),
//               policy.Principal("AWS", account1Arn, account2Arn),
//               policy.Principal("AWS", otherArns),
//               policy.Resource(
//                   pulumi.Sprintf("%s/*", newBucket.BucketArn),
//               ),
//           ),
//           policy.Statement("cloudfront-access",
//               policy.Effect(policy.Allow),
//               policy.Action("s3:GetObject"),
//               policy.Principal("AWS", cloudfrontOAI.IamARN),
//               policy.Resource(
//                   pulumi.Sprintf("%s/*", newBucket.BucketArn),
//               ),
//            ),
//         ).ToStringOutput(),
//      })
package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Package errors returned during validation of policies and statements.
var (
	ErrInvalidPolicy    = errors.New("invalid policy")
	ErrInvalidStatement = errors.New("invalid statement")
)

// EffectType is used with the Effect element of a Statement.
type EffectType string

// Valid effects for use with Statements.
const (
	Allow EffectType = "Allow"
	Deny  EffectType = "Deny"
)

// Policy defines an IAM policy that can be converted to a JSON
// StringOutput.
//
// It accepts strings or StringInputs for parameters and will use ApplyT to
// resolve any dependent inputs before generating the JSON.
type Policy struct {
	Version   string
	ID        string `json:"Id"`
	Statement Stmts
}

// Validate performs a basic structural check of the Policy.
func (p Policy) Validate() error {
	if p.Version == "" || p.ID == "" {
		return fmt.Errorf("%w: policy %q has no version or id set", ErrInvalidPolicy, p.ID)
	}
	for _, s := range p.Statement {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("policy %q has errors: %w", p.ID, err)
		}
	}
	return nil
}

// ToStringOutput generates a formatted JSON policy as a suitable input
// for various AWS objects that require one.
func (p Policy) ToStringOutput() pulumi.StringOutput {
	return p.ToStringOutputWithContext(context.Background())
}

// ToStringOutputWithContext generates a formatted JSON policy as a suitable input
// for various AWS objects that require one.
func (p Policy) ToStringOutputWithContext(ctx context.Context) pulumi.StringOutput {
	if err := p.Validate(); err != nil {
		panic(err)
	}
	return pulumi.ToOutput(p).ApplyTWithContext(ctx, func(_, data interface{}) string {
		v, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			panic(fmt.Sprintf("failed to marshal json for policy %q: %v", p.ID, err))
		}
		return string(v)
	}).(pulumi.StringOutput)
}

// Stmts holds an ordered group of statements.
type Stmts []Stmt

// Stmt define a single policy statement.
type Stmt struct {
	Sid          string `json:",omitempty"`
	Effect       EffectType
	Principal    map[string]Strings            `json:",omitempty"`
	NotPrincipal map[string]Strings            `json:",omitempty"`
	Action       Strings                       `json:",omitempty"`
	NotAction    Strings                       `json:",omitempty"`
	Resource     Strings                       `json:",omitempty"`
	NotResource  Strings                       `json:",omitempty"`
	Condition    map[string]map[string]Strings `json:",omitempty"`
}

// Validate does some very basic checks to ensure required fields are present.
func (s Stmt) Validate() error {
	if s.Effect != Allow && s.Effect != Deny {
		return fmt.Errorf("%w: invalid Effect element for statement %q: %q",
			ErrInvalidStatement, s.Sid, s.Effect)
	}
	if len(s.Principal) > 0 && len(s.NotPrincipal) > 0 {
		return fmt.Errorf("%w: Principal and NotPrincipal are mutually exclusive for statement %q",
			ErrInvalidStatement, s.Sid)
	}
	if len(s.Action) == 0 && len(s.NotAction) == 0 {
		return fmt.Errorf("%w: no Action or NotAction specified for statement %q",
			ErrInvalidStatement, s.Sid)
	}
	if len(s.Action) > 0 && len(s.NotAction) > 0 {
		return fmt.Errorf("%w: Action and NotAction are mutually exclusive for statement %q",
			ErrInvalidStatement, s.Sid)
	}
	if len(s.Resource) > 0 && len(s.NotResource) > 0 {
		return fmt.Errorf("%w: Resource and NotResource are mutually exclusive for statement %q",
			ErrInvalidStatement, s.Sid)
	}
	return nil

}

// Strings is a convenience helper that marshals its entries either to a
// JSON array, or a single string if only one item is in the list.
type Strings []interface{}

// MarshalJSON implements json.Marshaler.
func (s Strings) MarshalJSON() ([]byte, error) {
	entries := s.flatten()
	if len(entries) == 1 {
		return json.Marshal(entries[0])
	}
	return json.Marshal(entries)
}

func (s Strings) flatten() []string {
	out := make([]string, 0, len(s))
	for _, el := range s {
		switch v := el.(type) {
		case string:
			out = append(out, v)
		case []string:
			out = append(out, v...)
		default:
			panic(fmt.Sprintf("unexpected type passed to flatten: %T: %#v", el, el))
		}
	}
	return out
}

// Opt is implemented by functions that can be passed to New.
type Opt func(*Policy)

// New creates a new Policy with the supplied ID.  It should supply at least
// a single Statement as an argument.
func New(id string, opts ...Opt) *Policy {
	p := &Policy{
		Version: "2012-10-17",
		ID:      id,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// StatementOpt is implemented by functions that can be passed to Statement.
type StatementOpt func(*Stmt)

// Statement defines a single policy statement that can be passed to New.
func Statement(sid string, opts ...StatementOpt) Opt {
	return func(p *Policy) {
		s := Stmt{
			Sid:          sid,
			Principal:    map[string]Strings{},
			NotPrincipal: map[string]Strings{},
		}
		for _, opt := range opts {
			opt(&s)
		}
		p.Statement = append(p.Statement, s)
	}
}

// Effect specifies whether a Statement has an Allow or Deny effect.
func Effect(effect EffectType) StatementOpt {
	return func(s *Stmt) {
		s.Effect = effect
	}
}

// Principal adds one or more principals to the Principal element of a Statement.
// It can be called multiple times to add additional principals.
//
// principalType should be one of "AWS", "CanonicalUser", etc.
// prinicpalID arguments may be string, []string, StringInput or StrayArrayInput
// slices and arrays will be flattened into a single list.
func Principal(principalType string, principalID ...interface{}) StatementOpt {
	return func(s *Stmt) {
		s.Principal[principalType] = append(s.Principal[principalType], principalID...)
	}
}

// NotPrincipal adds one or more principals to the NotPrincipal element of a Statement.
// It can be called multiple times to add additional principals.
//
// principalType should be one of "AWS", "CanonicalUser", etc.
// prinicpalID arguments may be string, []string, StringInput or StrayArrayInput
// slices and arrays will be flattened into a single list.
func NotPrincipal(principalType string, principalID ...interface{}) StatementOpt {
	return func(s *Stmt) {
		s.NotPrincipal[principalType] = append(s.NotPrincipal[principalType], principalID...)
	}
}

// Action adds one or more entries to the Action element of a Statement.
// It can be called multiple times to add additional actions.
//
// action arguments may be string, []string, StringInput or StrayArrayInput
// slices and arrays will be flattened into a single list.
func Action(action ...interface{}) StatementOpt {
	return func(s *Stmt) {
		s.Action = append(s.Action, action...)
	}
}

// NotAction adds one or more entries to the NotAction element of a Statement.
// It can be called multiple times to add additional actions.
//
// action arguments may be string, []string, StringInput or StrayArrayInput
// slices and arrays will be flattened into a single list.
func NotAction(action ...interface{}) StatementOpt {
	return func(s *Stmt) {
		s.NotAction = append(s.NotAction, action...)
	}
}

// Resource adds one or more entries to the Resource element of a Statement.
// It can be called multiple times to add additional resources.
//
// resource arguments may be string, []string, StringInput or StrayArrayInput
// slices and arrays will be flattened into a single list.
func Resource(resource ...interface{}) StatementOpt {
	return func(s *Stmt) {
		s.Resource = append(s.Resource, resource...)
	}
}

// NotResource adds one or more entries to the NotResource element of a Statement.
// It can be called multiple times to add additional resources.
//
// resource arguments may be string, []string, StringInput or StrayArrayInput
// slices and arrays will be flattened into a single list.
func NotResource(resource ...interface{}) StatementOpt {
	resources := resource
	return func(s *Stmt) {
		s.NotResource = append(s.NotResource, resources...)
	}
}

// Condition adds an entry to the Condition element of a Statemment.
// It can be called multiple times to add additional resources.
//
// See https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_condition.html
// for examples of condition operators, keys and values.
//
// conditionValues arguments may be string, []string, StringInput or StrayArrayInput.
func Condition(conditionOp, conditionKey string, conditionValue ...interface{}) StatementOpt {
	return func(s *Stmt) {
		if s.Condition == nil {
			s.Condition = make(map[string]map[string]Strings)
		}
		if s.Condition[conditionOp] == nil {
			s.Condition[conditionOp] = make(map[string]Strings)
		}
		var v Strings
		v = append(v, conditionValue...)
		s.Condition[conditionOp][conditionKey] = v
	}
}
