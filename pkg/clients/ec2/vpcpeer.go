package ec2

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/smithy-go"
)

const (
	// VPCPeeringConnectionNotFound is the code that is returned by ec2 when the given VPCPeeringConnectionID is not valid
	VPCPeeringConnectionNotFound = "InvalidVpcPeeringConnectionID.NotFound"
)

type VPCPeerAccepterClient interface {
	AcceptVpcPeeringConnection(ctx context.Context, params *ec2.AcceptVpcPeeringConnectionInput, optFns ...func(*ec2.Options)) (*ec2.AcceptVpcPeeringConnectionOutput, error)
	DescribeVpcPeeringConnections(ctx context.Context, params *ec2.DescribeVpcPeeringConnectionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcPeeringConnectionsOutput, error)
}

// NewVPCPeerAccepterClient returns a new client using AWS credentials as JSON encoded data.
func NewVPCPeerAccepterClient(cfg aws.Config) VPCPeerAccepterClient {
	return ec2.NewFromConfig(cfg)
}

// IsVPCPeeringConnectionNotFoundErr returns true if the error is because the item doesn't exist
func IsVPCPeeringConnectionNotFoundErr(err error) bool {
	var awsErr smithy.APIError
	return errors.As(err, &awsErr) && awsErr.ErrorCode() == VPCPeeringConnectionNotFound
}
