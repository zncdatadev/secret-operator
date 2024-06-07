package kerberos

import (
	"crypto/sha256"
	"os"
	"os/exec"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	kadminLogger = ctrl.Log.WithName("kadmin")
)

type Kadmin struct {
	// ref: https://web.mit.edu/kerberos/krb5-latest/doc/admin/conf_files/kadm5_acl.html#kadm5-acl-5
	// Admin user must have permission with "xe" in kadm5.acl
	adminPrincipal *string
	adminKeytab    []byte
	krb5Config     *Krb5Config

	// adminKeytabPath is the path of the keytab file
	// if the field is empty, it will generate a temporary keytab file with hashed name
	// if the field is not empty, it will use the existing keytab file,
	// 	when the file is not found, it will create the file.
	adminKeytabPath string
}

func NewKadmin(
	krb5Config *Krb5Config,
	adminPrincipal *string,
	adminKeytab []byte,
) *Kadmin {
	return &Kadmin{
		krb5Config:     krb5Config,
		adminPrincipal: adminPrincipal,
		adminKeytab:    adminKeytab,
	}
}

func (k *Kadmin) GetAdminPrincipal() *string {
	return k.adminPrincipal
}

// GetAdminKeytabPath returns the path of the admin keytab file.
// If the admin keytab path is empty, it creates a new temp keytab file and returns its path.
// If the admin keytab path already exists, it checks if the file exists and creates a new keytab file if it doesn't.
// It returns the admin keytab path and any error encountered during the process.
func (k *Kadmin) GetAdminKeytabPath() (string, error) {
	if k.adminKeytabPath == "" {
		hash := sha256.New()
		hash.Write(k.adminKeytab)
		keytabPath := os.TempDir() + "/admin-keytab-" + string(hash.Sum(nil)[:24]) + ".keytab"

		keytab, err := os.Create(keytabPath)
		if err != nil {
			kadminLogger.Error(err, "Failed to create keytab file")
			return "", err
		}

		if _, err := keytab.Write(k.adminKeytab); err != nil {
			kadminLogger.Error(err, "Failed to write keytab")
			return "", err
		}
		k.adminKeytabPath = keytab.Name()
	} else if _, err := os.Stat(k.adminKeytabPath); os.IsNotExist(err) {
		keytab, err := os.Create(k.adminKeytabPath)
		if err != nil {
			kadminLogger.Error(err, "Failed to create keytab file")
			return "", err
		}

		if _, err := keytab.Write(k.adminKeytab); err != nil {
			kadminLogger.Error(err, "Failed to write keytab")
			return "", err
		}
	}

	return k.adminKeytabPath, nil
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
	krb5Path, err := k.krb5Config.GetTempPath()
	if err != nil {
		kadminLogger.Error(err, "Failed to get krb5 path")
		return "", err
	}

	adminKeytabPath, err := k.GetAdminKeytabPath()
	if err != nil {
		kadminLogger.Error(err, "Failed to get admin keytab path")
		return "", err
	}

	cmd := exec.Command("kadmin", "-kt", adminKeytabPath, "-p", *k.GetAdminPrincipal(), "query", query)
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

	// When execute: kadmin -kt /tmp/foo/admin.keytab -p admin/admin -q "addprinc -randkey foo"
	// Added output:
	// 	Authenticating as principal admin/admin with keytab /tmp/foo/admin.keytab.
	// 	No policy specified for foo@EXAMPLE.COM; defaulting to no policy
	// 	Principal "foo@EXAMPLE.COM" created.
	// exit code 0
	//
	// Existing output:
	// 	Authenticating as principal admin/admin with keytab /tmp/foo/admin.keytab.
	// 	No policy specified for foo@EXAMPLE.COM; defaulting to no policy
	// 	add_principal: Principal or policy already exists while creating "foo@EXAMPLE.COM".
	// exit code 0
	//
	output, err := k.Query(strings.Join(queries, " "))
	if err != nil {
		kadminLogger.Error(err, "Failed to add principal", "principal", principal)
		return err
	}

	kadminLogger.Info("Added principal", "principal", principal, "output", output)

	return nil
}
