/*
Copyright 2021 The Crossplane Authors.

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

package vpcpeerconnectionaccepter

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	svcapitypes "github.com/crossplane-contrib/provider-aws/apis/ec2/manual2v1alpha1"
	"github.com/crossplane-contrib/provider-aws/apis/v1alpha1"
	awsclient "github.com/crossplane-contrib/provider-aws/pkg/clients"
	"github.com/crossplane-contrib/provider-aws/pkg/clients/ec2"
	"github.com/crossplane-contrib/provider-aws/pkg/features"
)

const (
	errUnexpectedObject = "The managed resource is not a VPCPeerConnectionAccepter resource"

	errDescribe = "failed to describe VPCPeeringConnections with id"
	errAccept   = "failed toa ccept the peering connection"
)

// SetupVPCPeerConnectionAccepter adds a controller that reconciles VPCPeerConnectionAccepter.
func SetupVPCPeerConnectionAccepter(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(svcapitypes.VPCPeerConnectionAccepterGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), v1alpha1.StoreConfigGroupVersionKind))
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&svcapitypes.VPCPeerConnectionAccepter{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(svcapitypes.VPCPeerConnectionAccepterGroupVersionKind),
			managed.WithExternalConnecter(&connector{kube: mgr.GetClient(), newClientFn: ec2.NewVPCPeerAccepterClient}),
			managed.WithReferenceResolver(managed.NewAPISimpleReferenceResolver(mgr.GetClient())),
			managed.WithConnectionPublishers(),
			managed.WithInitializers(managed.NewDefaultProviderConfig(mgr.GetClient())),
			managed.WithPollInterval(o.PollInterval),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
			managed.WithConnectionPublishers(cps...)))
}

type connector struct {
	kube        client.Client
	newClientFn func(config aws.Config) ec2.VPCPeerAccepterClient
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*svcapitypes.VPCPeerConnectionAccepter)
	if !ok {
		return nil, errors.New(errUnexpectedObject)
	}
	cfg, err := awsclient.GetConfig(ctx, c.kube, mg, cr.Spec.ForProvider.Region)
	if err != nil {
		return nil, err
	}
	return &external{client: c.newClientFn(*cfg), kube: c.kube}, nil
}

type external struct {
	kube   client.Client
	client ec2.VPCPeerAccepterClient
}

func (e *external) Observe(ctx context.Context, mgd resource.Managed) (managed.ExternalObservation, error) { // nolint:gocyclo
	cr, ok := mgd.(*svcapitypes.VPCPeerConnectionAccepter)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errUnexpectedObject)
	}

	if meta.GetExternalName(cr) == "" {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	response, err := e.client.DescribeVpcPeeringConnections(ctx,
		&awsec2.DescribeVpcPeeringConnectionsInput{
			VpcPeeringConnectionIds: []string{meta.GetExternalName(cr)},
		})
	if err != nil {
		return managed.ExternalObservation{}, awsclient.Wrap(resource.Ignore(ec2.IsVPCPeeringConnectionNotFoundErr, err), errDescribe)
	}

	if len(response.VpcPeeringConnections) == 0 {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	observed := response.VpcPeeringConnections[0]
	code := observed.Status.Code
	switch code {
	case types.VpcPeeringConnectionStateReasonCodeProvisioning:
		fallthrough
	case types.VpcPeeringConnectionStateReasonCodeInitiatingRequest:
		fallthrough
	case types.VpcPeeringConnectionStateReasonCodePendingAcceptance:
		cr.SetConditions(xpv1.Creating())
	case types.VpcPeeringConnectionStateReasonCodeActive:
		cr.SetConditions(xpv1.Available())
	case types.VpcPeeringConnectionStateReasonCodeDeleting:
		cr.SetConditions(xpv1.Deleting())
	case types.VpcPeeringConnectionStateReasonCodeDeleted:
		return managed.ExternalObservation{}, nil
	}

	cr.Status.AtProvider.Status = aws.String(string(code))
	cr.Status.AtProvider.Message = observed.Status.Message
	if observed.ExpirationTime != nil {
		expTime := metav1.NewTime(aws.ToTime(observed.ExpirationTime))
		cr.Status.AtProvider.ExpirationTime = &expTime
	}

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        code == types.VpcPeeringConnectionStateReasonCodeActive,
		ResourceLateInitialized: false,
	}, nil
}

func (e *external) Create(ctx context.Context, mgd resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mgd.(*svcapitypes.VPCPeerConnectionAccepter)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errUnexpectedObject)
	}

	result, err := e.client.AcceptVpcPeeringConnection(ctx, &awsec2.AcceptVpcPeeringConnectionInput{
		VpcPeeringConnectionId: cr.Spec.ForProvider.VPCPeeringConnectionID,
	})
	if err != nil {
		return managed.ExternalCreation{}, awsclient.Wrap(err, errAccept)
	}

	meta.SetExternalName(cr, awsclient.StringValue(result.VpcPeeringConnection.VpcPeeringConnectionId))

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mgd resource.Managed) (managed.ExternalUpdate, error) { // nolint:gocyclo
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mgd resource.Managed) error {
	cr, ok := mgd.(*svcapitypes.VPCPeerConnectionAccepter)
	if !ok {
		return errors.New(errUnexpectedObject)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	return nil
}
