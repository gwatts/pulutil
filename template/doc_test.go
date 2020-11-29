package template_test

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/gwatts/pulutil/template"

	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

type mocks int

func (mocks) NewResource(typeToken, name string, inputs resource.PropertyMap, provider, id string) (string, resource.PropertyMap, error) {
	return name + "_id", inputs, nil
}

func (mocks) Call(token string, args resource.PropertyMap, provider string) (resource.PropertyMap, error) {
	return args, nil
}

func ExampleNew() {
	someStringOutput := pulumi.String("later-string").ToStringOutput()

	templateOutput := template.New(map[string]interface{}{
		"Foo":    "bar",
		"SomeID": someStringOutput,
	}, `
		"Foo" is set to "{{ .Foo }}"
        "SomeID" is set to "{{ .SomeID }}"
	`)

	// templateOutput can now be supplied to a resource etc
	pulumi.Sprintf("template rendered to: %s", templateOutput)
}

func ExampleNewJSON() {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// create a new bucket - It's bucket.Arn field won't be known
		// until it's created, so is an output.
		bucket, err := s3.NewBucket(ctx, "mybucket", &s3.BucketArgs{})
		if err != nil {
			return err
		}

		// Grant Cloudfront access to the bucket
		p, err := s3.NewBucketPolicy(ctx, "bucketPolicy", &s3.BucketPolicyArgs{
			Bucket: bucket.Bucket,
			Policy: template.NewJSON(map[string]interface{}{
				// parameters we want to pass to the template
				"BucketARN":   bucket.Arn,
				"IdentityARN": "my-identity",
			}, `{
						"Version": "2012-10-17",
						"Id": "PolicyForCloudFrontContent",
						"Statement": [
							{
								"Effect": "Allow",
								"Principal": {
									"AWS": "{{ .IdentityARN }}"
								},
								"Action": "s3:GetObject",
								"Resource": "{{ .BucketARN }}/*"
							}
						]
					}`),
		})
		if err != nil {
			return err
		}

		// The following is just a test to output the rendered template
		var wg sync.WaitGroup
		wg.Add(1)
		pulumi.All(p.Policy).ApplyT(func(all []interface{}) error {
			defer wg.Done()
			policy := all[0].(string)
			// strip the leading whitespace from each line for easier comparison
			var strip = regexp.MustCompile(`(?m)^\s+`)
			fmt.Println(strip.ReplaceAllLiteralString(policy, ""))
			return nil
		})
		wg.Wait()
		return nil
	}, pulumi.WithMocks("project", "stack", mocks(0)))

	if err != nil {
		panic(fmt.Errorf("unexpected error: %v", err))
	}

	// Output:
	//{
	//"Version": "2012-10-17",
	//"Id": "PolicyForCloudFrontContent",
	//"Statement": [
	//{
	//"Effect": "Allow",
	//"Principal": {
	//"AWS": "my-identity"
	//},
	//"Action": "s3:GetObject",
	//"Resource": "/*"
	//}
	//]
	//}

}
