package vault

import (
	"context"
	"errors"
	svcsdkapi "github.com/aws/aws-sdk-go/service/glacier/glacieriface"
	"github.com/google/go-cmp/cmp"

	"github.com/aws/aws-sdk-go/aws"
	svcsdk "github.com/aws/aws-sdk-go/service/glacier"
	"github.com/crossplane-contrib/provider-aws/apis/v1alpha1"
	"github.com/crossplane-contrib/provider-aws/pkg/features"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	ctrl "sigs.k8s.io/controller-runtime"

	svcapitypes "github.com/crossplane-contrib/provider-aws/apis/glacier/v1alpha1"
)

func SetupVault(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(svcapitypes.VaultGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), v1alpha1.StoreConfigGroupVersionKind))
	}

	opts := []option{
		func(e *external) {
			e.preObserve = preObserve
			e.preCreate = preCreate
			e.preDelete = preDelete
			e.postObserve = postObserve
			u := updateClient{client: e.client}
			e.update = u.update
			e.isUpToDate = u.isUpToDate
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&svcapitypes.Vault{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(svcapitypes.VaultGroupVersionKind),
			managed.WithExternalConnecter(&connector{kube: mgr.GetClient(), opts: opts}),
			managed.WithPollInterval(o.PollInterval),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
			managed.WithConnectionPublishers(cps...)))
}

func preObserve(_ context.Context, cr *svcapitypes.Vault, obj *svcsdk.DescribeVaultInput) error {
	obj.VaultName = aws.String(meta.GetExternalName(cr))
	return nil
}

func preCreate(_ context.Context, cr *svcapitypes.Vault, obj *svcsdk.CreateVaultInput) error {
	obj.VaultName = aws.String(meta.GetExternalName(cr))
	return nil
}

func preDelete(_ context.Context, cr *svcapitypes.Vault, obj *svcsdk.DeleteVaultInput) (bool, error) {
	obj.VaultName = aws.String(meta.GetExternalName(cr))
	return false, nil
}

func postObserve(_ context.Context, cr *svcapitypes.Vault, resp *svcsdk.DescribeVaultOutput, obs managed.ExternalObservation, err error) (managed.ExternalObservation, error) {
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	cr.SetConditions(xpv1.Available())
	return obs, nil
}

type updateClient struct {
	client svcsdkapi.GlacierAPI
}

func (u *updateClient) update(ctx context.Context, cr resource.Managed) (managed.ExternalUpdate, error) {
	vault, ok := cr.(*svcapitypes.Vault)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errUnexpectedObject)
	}
	vaultName := aws.String(meta.GetExternalName(cr))

	err := u.updateAccessPolicy(ctx, vault, vaultName)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	err = u.updateLockPolicy(ctx, vault, vaultName)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	err = u.updateTags(ctx, vault, vaultName)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	return managed.ExternalUpdate{}, nil
}

func (u *updateClient) updateAccessPolicy(ctx context.Context, vault *svcapitypes.Vault, vaultName *string) error {
	accessPolicy := vault.Spec.ForProvider.AccessPolicy
	if accessPolicy != nil {
		policyInput := &svcsdk.SetVaultAccessPolicyInput{
			VaultName: vaultName,
			AccountId: vault.Spec.ForProvider.AccountID,
			Policy:    &svcsdk.VaultAccessPolicy{Policy: accessPolicy.Policy},
		}
		_, err := u.client.SetVaultAccessPolicyWithContext(ctx, policyInput)
		if err != nil {
			return err
		}
	} else {
		policyResp, err := u.client.GetVaultAccessPolicyWithContext(ctx, &svcsdk.GetVaultAccessPolicyInput{
			VaultName: vaultName,
			AccountId: vault.Spec.ForProvider.AccountID,
		})
		if err != nil {
			return err
		}
		if policyResp.Policy != nil {
			_, err = u.client.DeleteVaultAccessPolicyWithContext(ctx, &svcsdk.DeleteVaultAccessPolicyInput{
				VaultName: vaultName,
				AccountId: vault.Spec.ForProvider.AccountID,
			})
		}
	}

	return nil
}

func (u *updateClient) updateLockPolicy(ctx context.Context, vault *svcapitypes.Vault, vaultName *string) error {
	if vault.Spec.ForProvider.LockPolicy == nil {
		return nil
	}
	getLockResp, err := u.client.GetVaultLockWithContext(ctx, &svcsdk.GetVaultLockInput{
		VaultName: vaultName,
		AccountId: vault.Spec.ForProvider.AccountID,
	})
	if IsNotFound(err) || !isLockSet(getLockResp) {
		_, err := u.client.InitiateVaultLockWithContext(ctx, &svcsdk.InitiateVaultLockInput{
			VaultName: vaultName,
			AccountId: vault.Spec.ForProvider.AccountID,
			Policy:    &svcsdk.VaultLockPolicy{Policy: vault.Spec.ForProvider.LockPolicy.Policy},
		})
		if err != nil {
			return err
		}
	} else {
		return err
	}
	return nil
}

func isLockSet(resp *svcsdk.GetVaultLockOutput) bool {
	if resp == nil {
		return false
	}
	return resp.Policy != nil
}

func (u *updateClient) updateTags(ctx context.Context, vault *svcapitypes.Vault, vaultName *string) error {
	awsTags := getAWSTags(vault)
	addTagsInput := &svcsdk.AddTagsToVaultInput{
		VaultName: vaultName,
		AccountId: vault.Spec.ForProvider.AccountID,
		Tags:      awsTags,
	}
	_, err := u.client.AddTagsToVaultWithContext(ctx, addTagsInput)
	if err != nil {
		return err
	}

	//remove extra tags
	listResp, err := u.client.ListTagsForVaultWithContext(ctx, &svcsdk.ListTagsForVaultInput{
		VaultName: vaultName,
		AccountId: vault.Spec.ForProvider.AccountID,
	})
	if err != nil {
		return err
	}

	var deleteKeys []*string
	for k, _ := range listResp.Tags {
		if _, found := awsTags[k]; !found {
			deleteKeys = append(deleteKeys, aws.String(k))
		}
	}
	_, err = u.client.RemoveTagsFromVaultWithContext(ctx, &svcsdk.RemoveTagsFromVaultInput{
		VaultName: vaultName,
		AccountId: vault.Spec.ForProvider.AccountID,
		TagKeys:   deleteKeys,
	})
	if err != nil {
		return err
	}
	return err
}

func getAWSTags(vault *svcapitypes.Vault) map[string]*string {
	awsTags := map[string]*string{}
	for k, v := range vault.Spec.ForProvider.Tags {
		awsTags[k] = aws.String(v)
	}
	return awsTags
}

func (u *updateClient) isUpToDate(vault *svcapitypes.Vault, output *svcsdk.DescribeVaultOutput) (bool, error) {
	vaultName := aws.String(meta.GetExternalName(vault))
	listResp, err := u.client.ListTagsForVault(&svcsdk.ListTagsForVaultInput{
		VaultName: vaultName,
		AccountId: vault.Spec.ForProvider.AccountID,
	})
	if err != nil {
		return false, err
	}
	if !cmp.Equal(listResp.Tags, getAWSTags(vault)) {
		return false, nil
	}

	if vault.Spec.ForProvider.LockPolicy != nil {
		getVaultLockResp, err := u.client.GetVaultLock(&svcsdk.GetVaultLockInput{
			VaultName: vaultName,
			AccountId: vault.Spec.ForProvider.AccountID,
		})
		if IsNotFound(err) || !isLockSet(getVaultLockResp) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
	}

	policyResp, err := u.client.GetVaultAccessPolicy(&svcsdk.GetVaultAccessPolicyInput{
		VaultName: vaultName,
		AccountId: vault.Spec.ForProvider.AccountID,
	})
	if err != nil && !IsNotFound(err) {
		return false, err
	}

	if !cmp.Equal(GenerateSDKAccessPolicy(vault.Spec.ForProvider.AccessPolicy), getPolicy(policyResp)) {
		return false, nil
	}
	return true, nil
}

func GenerateSDKAccessPolicy(in *svcapitypes.VaultAccessPolicy) *svcsdk.VaultAccessPolicy {
	if in == nil || in.Policy == nil {
		return nil
	}
	res := &svcsdk.VaultAccessPolicy{}
	res.SetPolicy(*in.Policy)
	return res
}

func getPolicy(policy *svcsdk.GetVaultAccessPolicyOutput) *svcsdk.VaultAccessPolicy {
	if policy == nil {
		return nil
	}

	return policy.Policy
}
