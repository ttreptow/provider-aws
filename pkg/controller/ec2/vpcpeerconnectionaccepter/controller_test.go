package vpcpeerconnectionaccepter

import (
	"context"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/crossplane-contrib/provider-aws/apis/ec2/manual2v1alpha1"
	"github.com/crossplane-contrib/provider-aws/pkg/clients/ec2"
	"github.com/crossplane-contrib/provider-aws/pkg/clients/ec2/fake"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"
)

var (
	expTime     = time.Now()
	metaExpTime = metav1.NewTime(expTime)
	testMessage = aws.String("ok")
	peerId      = "vpc-peer-123"
)

type args struct {
	accepter ec2.VPCPeerAccepterClient
	kube     client.Client
	cr       *manual2v1alpha1.VPCPeerConnectionAccepter
}

type peerAccepterModifier func(accepter *manual2v1alpha1.VPCPeerConnectionAccepter)

func withExternalName(name string) peerAccepterModifier {
	return func(r *manual2v1alpha1.VPCPeerConnectionAccepter) { meta.SetExternalName(r, name) }
}

func withConditions(c ...xpv1.Condition) peerAccepterModifier {
	return func(r *manual2v1alpha1.VPCPeerConnectionAccepter) { r.Status.ConditionedStatus.Conditions = c }
}

func withStatus(s manual2v1alpha1.VPCPeerConnectionAccepterObservation) peerAccepterModifier {
	return func(r *manual2v1alpha1.VPCPeerConnectionAccepter) { r.Status.AtProvider = s }
}

func peerAccepter(m ...peerAccepterModifier) *manual2v1alpha1.VPCPeerConnectionAccepter {
	cr := &manual2v1alpha1.VPCPeerConnectionAccepter{}
	for _, f := range m {
		f(cr)
	}
	return cr
}

func TestObserve(t *testing.T) {
	type want struct {
		cr     *manual2v1alpha1.VPCPeerConnectionAccepter
		result managed.ExternalObservation
		err    error
	}

	cases := map[string]struct {
		args
		want
	}{
		"Accepted": {
			args: args{
				accepter: &fake.MockVPCPeerAccepterClient{
					MockDescribe: func(ctx context.Context, params *awsec2.DescribeVpcPeeringConnectionsInput, optFns ...func(*awsec2.Options)) (*awsec2.DescribeVpcPeeringConnectionsOutput, error) {
						return &awsec2.DescribeVpcPeeringConnectionsOutput{
							VpcPeeringConnections: []types.VpcPeeringConnection{
								{
									Status: &types.VpcPeeringConnectionStateReason{
										Code:    types.VpcPeeringConnectionStateReasonCodeActive,
										Message: testMessage,
									},
									ExpirationTime: aws.Time(expTime),
								},
							},
						}, nil
					},
				},
				cr: peerAccepter(withStatus(manual2v1alpha1.VPCPeerConnectionAccepterObservation{
					Message:        nil,
					Status:         aws.String(string(types.VpcPeeringConnectionStateReasonCodePendingAcceptance)),
					ExpirationTime: nil,
				}), withExternalName(peerId)),
			},
			want: want{
				cr: peerAccepter(withStatus(manual2v1alpha1.VPCPeerConnectionAccepterObservation{
					Message:        testMessage,
					Status:         aws.String(string(types.VpcPeeringConnectionStateReasonCodeActive)),
					ExpirationTime: &metaExpTime,
				}), withExternalName(peerId), withConditions(xpv1.Available())),
				result: managed.ExternalObservation{
					ResourceExists:   true,
					ResourceUpToDate: true,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{kube: tc.kube, client: tc.accepter}
			o, err := e.Observe(context.Background(), tc.args.cr)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cr, tc.args.cr, test.EquateConditions()); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.result, o); diff != "" {
				t.Errorf("r: -want, +got:\n%s", diff)
			}
		})
	}
}
