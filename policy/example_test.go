package policy_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/gwatts/pulutil/policy"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/require"
)

type mocks int

func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name + "_id", args.Inputs, nil
}

func (mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

func ExampleNew() {
	var wg sync.WaitGroup
	_ = pulumi.RunErr(func(ctx *pulumi.Context) error {
		// bucketArn might be an output from a prior bucket creation
		bucketArn := pulumi.String("arn:aws:s3:::my-test-bucket-1234abcd")

		// Create a BucketPolicy using the above policy definition
		bp, _ := s3.NewBucketPolicy(ctx, "test-policy", &s3.BucketPolicyArgs{
			Bucket: pulumi.String("my-test-bucket-1234abcd"),
			Policy: policy.New("my-policy",
				policy.Statement("statement-one",
					policy.Effect(policy.Allow),
					policy.Action(
						"s3:GetObject",
						"s3:PutObject",
					),
					policy.Principal("AWS",
						"arn:aws:iam::12345:root",
					),
					policy.Resource(
						pulumi.Sprintf("%s/*", bucketArn),
					),
				),
			).ToStringOutput(),
		})

		// Print out the JSON that was computed
		wg.Add(1)
		bp.Policy.ApplyT(func(bp string) int {
			fmt.Println(bp)
			wg.Done()
			return 0
		})
		return nil
	}, pulumi.WithMocks("project", "stack", mocks(0)))
	wg.Wait()
	// Output:
	// {
	//     "Version": "2012-10-17",
	//     "Id": "my-policy",
	//     "Statement": {
	//         "Sid": "statement-one",
	//         "Effect": "Allow",
	//         "Principal": {
	//             "AWS": "arn:aws:iam::12345:root"
	//         },
	//         "Action": [
	//             "s3:GetObject",
	//             "s3:PutObject"
	//         ],
	//         "Resource": "arn:aws:s3:::my-test-bucket-1234abcd/*"
	//     }
	// }
}

func TestPolicy(t *testing.T) {
	require := require.New(t)
	p := policy.New("id")
	require.NotNil(p)
}
