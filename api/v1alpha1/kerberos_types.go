package v1alpha1

type KerberosKeytabSpec struct {
	Admin             *AdminServerSpec  `json:"admin"`
	AdminPrincipal    string            `json:"adminPrincipal"`
	AdminKeytabSecret *KeytabSecretSpec `json:"adminKeytabSecret"`
	KDC               string            `json:"kdc"`

	// +kubebuilder:validation:Pattern=`^[-.a-zA-Z0-9]+$`
	RealmName string `json:"realmName"`
}

type KeytabSecretSpec struct {
	// +kubebuilder:validation:Required
	// Contains the `keytab` name of the secret
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

type AdminServerSpec struct {
	// MIT kerberos admin server.
	// +kubebuilder:validation:Required
	MIT *MITSpec `json:"mit"`

	// MS-AD
}

type MITSpec struct {
	// The hostname of the kadmin server.
	// +kubebuilder:validation:Required
	KadminServer string `json:"kadminServer"`
}
