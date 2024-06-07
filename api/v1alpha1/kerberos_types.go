package v1alpha1

type KerberosKeytabSpec struct {
	AdminServer       *AdminServerSpec  `json:"adminServer"`
	AdminPrincipal    string            `json:"adminPrincipal"`
	AdminKeytabSecret *KeytabSecretSpec `json:"adminKeytabSecret"`
	KDC               string            `json:"kdc"`
	Realm             string            `json:"realm"`
}

type KeytabSecretSpec struct {
	// +kubebuilder:validation:Required
	// Contains the `keytab` name of the secret
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type AdminServerSpec struct {
	MIT *MITSpec `json:"mit"`

	// openldap
	// MS-AD
}

type MITSpec struct {
	KadminServer string `json:"kadminServer"`
}
