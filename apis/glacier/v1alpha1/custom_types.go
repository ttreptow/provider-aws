package v1alpha1

type CustomVaultParameters struct {
	//Set the access policy on the Vault
	AccessPolicy *VaultAccessPolicy `json:"accessPolicy,omitempty"`

	// Sets the Lock Policy on the Vault. This can only be set once and cannot be reversed
	// +immutable
	LockPolicy *VaultLockPolicy `json:"lockPolicy,omitempty"`

	//Tags applied to the Vault
	Tags map[string]string `json:"tags,omitempty"`
}
