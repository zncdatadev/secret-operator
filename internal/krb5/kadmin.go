package krb5

import (
	"os"
	"os/exec"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	kadminLogger = ctrl.Log.WithName("kadmin")
)

type User struct {
	Principal string
	Keytab    string
}

type Kadmin struct {
	// ref: https://web.mit.edu/kerberos/krb5-latest/doc/admin/conf_files/kadm5_acl.html#kadm5-acl-5
	// Admin user must have permission with "xe" in kadm5.acl
	Admin      User
	krb5Config Krb5Config
}

func NewKadmin(
	krb5Config Krb5Config,
	admin User,
) *Kadmin {
	return &Kadmin{
		Admin: User{
			Principal: admin.Principal,
			Keytab:    admin.Keytab,
		},
	}
}

// Query executes a kadmin query
// Example:
//
//	kadmin -kt admin.keytab -p admin/admin query "listprincs"
//	kadmin -kt admin.keytab -p admin/admin query "ktadd -k user1.keytab -norandkey user1"
//
// Node:
//
//	When generating keytab file, use "-norandkey" flag, the admin user must
//	have "e" permission in kadm5.acl.
func (k *Kadmin) Query(query string) (result string, err error) {
	krb5Path := k.krb5Config.GetTempPath()
	cmd := exec.Command("kadmin", "-kt", k.Admin.Keytab, "-p", k.Admin.Principal, "query", query)
	// https://web.mit.edu/kerberos/krb5-latest/doc/admin/install_kdc.html#edit-kdc-configuration-files
	cmd.Env = append(os.Environ(), "KRB5_CONFIG="+krb5Path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		kadminLogger.Error(err, "Failed to execute kadmin query", "cmd", cmd.String())
		return "", err
	}

	result = string(output)

	kadminLogger.Info("Executed kadmin query", "cmd", cmd.String(), "output", result)

	return result, nil

}

// Ktadd generates a keytab file for the given principals
// Usage: ktadd [-k[eytab] keytab] [-q] [-e keysaltlist] [-norandkey] [principal | -glob princ-exp] [...]
func (k *Kadmin) Ktadd(noRandKey bool, principals ...string) ([]byte, error) {

	keytab, err := os.CreateTemp("", "keytab-")
	if err != nil {
		kadminLogger.Error(err, "Failed to create keytab file")
		return nil, err
	}

	defer os.Remove(keytab.Name())

	queries := []string{
		"ktadd",
		"-k", // "-k" flag is used to specify the keytab file
		keytab.Name(),
	}

	if noRandKey {
		queries = append(queries, "-norandkey")
	}

	queries = append(queries, principals...)

	output, err := k.Query(strings.Join(queries, " "))
	if err != nil {
		kadminLogger.Error(err, "Failed to save keytab", "principals", principals, "keytab", keytab)
		return nil, err
	}

	kadminLogger.Info("Saved keytab", "principal", principals, "keytab", keytab, "output", output)

	return os.ReadFile(keytab.Name())
}

// AddPrincipal adds a new principal
// usage: https://web.mit.edu/kerberos/krb5-latest/doc/admin/admin_commands/kadmin_local.html#add-principal
func (k *Kadmin) AddPrincipal(principal string) error {
	queries := []string{
		"addprinc",
		"-randkey", // "-randkey" flag is used to generate a random key for the principal
		principal,
	}
	output, err := k.Query(strings.Join(queries, " "))
	if err != nil {
		kadminLogger.Error(err, "Failed to add principal", "principal", principal)
		return err
	}

	kadminLogger.Info("Added principal", "principal", principal, "output", output)

	return nil
}
