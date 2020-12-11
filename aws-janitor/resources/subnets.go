/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Subnets: https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#EC2.DescribeSubnets
type Subnets struct{}

func (Subnets) MarkAndSweep(opts Options, set *Set) error {
	logger := logrus.WithField("options", opts)
	svc := ec2.New(opts.Session, aws.NewConfig().WithRegion(opts.Region))

	descReq := &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("defaultForAz"),
				Values: []*string{aws.String("false")},
			},
		},
	}

	resp, err := svc.DescribeSubnets(descReq)
	if err != nil {
		return err
	}

	for _, sub := range resp.Subnets {
		s := &subnet{Account: opts.Account, Region: opts.Region, ID: *sub.SubnetId}
		if set.Mark(s, nil) {
			logger.Warningf("%s: deleting %T: %s", s.ARN(), sub, s.ID)
			if opts.DryRun {
				continue
			}
			if _, err := svc.DeleteSubnet(&ec2.DeleteSubnetInput{SubnetId: sub.SubnetId}); err != nil {
				logger.Warningf("%s: delete failed: %v", s.ARN(), err)
			}
		}
	}

	return nil
}

func (Subnets) ListAll(opts Options) (*Set, error) {
	svc := ec2.New(opts.Session, aws.NewConfig().WithRegion(opts.Region))
	set := NewSet(0)
	input := &ec2.DescribeSubnetsInput{}

	// Subnets not paginated
	subnets, err := svc.DescribeSubnets(input)
	now := time.Now()
	for _, sn := range subnets.Subnets {
		arn := subnet{
			Account: opts.Account,
			Region:  opts.Region,
			ID:      *sn.SubnetId,
		}.ARN()
		set.firstSeen[arn] = now
	}

	return set, errors.Wrapf(err, "couldn't describe subnets for %q in %q", opts.Account, opts.Region)
}

type subnet struct {
	Account string
	Region  string
	ID      string
}

func (sub subnet) ARN() string {
	return fmt.Sprintf("arn:aws:ec2:%s:%s:subnet/%s", sub.Region, sub.Account, sub.ID)
}

func (sub subnet) ResourceKey() string {
	return sub.ARN()
}
