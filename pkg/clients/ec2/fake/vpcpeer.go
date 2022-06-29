/*
Copyright 2019 The Crossplane Authors.

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

package fake

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	clientset "github.com/crossplane-contrib/provider-aws/pkg/clients/ec2"
)

// this ensures that the mock implements the client interface
var _ clientset.VPCPeerAccepterClient = (*MockVPCPeerAccepterClient)(nil)

// MockVPCPeerAccepterClient is a type that implements all the methods for VPCPeerAccepterClient interface
type MockVPCPeerAccepterClient struct {
	MockDescribe func(ctx context.Context, params *ec2.DescribeVpcPeeringConnectionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcPeeringConnectionsOutput, error)
	MockAccept   func(ctx context.Context, params *ec2.AcceptVpcPeeringConnectionInput, optFns ...func(*ec2.Options)) (*ec2.AcceptVpcPeeringConnectionOutput, error)
}

// AcceptVpcPeeringConnection mocks AcceptVpcPeeringConnection method
func (m *MockVPCPeerAccepterClient) AcceptVpcPeeringConnection(ctx context.Context, params *ec2.AcceptVpcPeeringConnectionInput, optFns ...func(*ec2.Options)) (*ec2.AcceptVpcPeeringConnectionOutput, error) {
	return m.MockAccept(ctx, params, optFns...)
}

// DescribeVpcPeeringConnections mocks DescribeVpcPeeringConnections method
func (m *MockVPCPeerAccepterClient) DescribeVpcPeeringConnections(ctx context.Context, params *ec2.DescribeVpcPeeringConnectionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcPeeringConnectionsOutput, error) {
	return m.MockDescribe(ctx, params, optFns...)
}
