package kerberos

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"

	"github.com/google/uuid"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	logger = ctrl.Log.WithName("kadmin")
	mutex  sync.Mutex
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
// If the admin keytab path is empty, use the temporary keytab file.
// If the admin keytab path already exists, it checks if the file exists and creates a new keytab file if it doesn't.
// It returns the admin keytab path and any error encountered during the process.
func (k *Kadmin) GetAdminKeytabPath() (string, error) {
	if k.adminKeytabPath == "" {
		keytabFile, err := os.CreateTemp("", "admin-keytab-*.keytab")
		if err != nil {
			logger.Error(err, "Failed to create temporary keytab file")
			return "", err
		}

		if _, err := keytabFile.Write(k.adminKeytab); err != nil {
			logger.Error(err, "Failed to write keytab")
			return "", err
		}
		k.adminKeytabPath = keytabFile.Name()
	} else if _, err := os.Stat(k.adminKeytabPath); os.IsNotExist(err) {
		keytab, err := os.Create(k.adminKeytabPath)
		if err != nil {
			logger.Error(err, "Failed to create keytab file")
			return "", err
		}

		if _, err := keytab.Write(k.adminKeytab); err != nil {
			logger.Error(err, "Failed to write keytab")
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
// Note:
//
//	When generating keytab file, use "-norandkey" flag, the admin user must
//	have "e" permission in kadm5.acl.
func (k *Kadmin) Query(query string) (result string, err error) {
	krb5Path, err := k.krb5Config.GetTempPath()
	if err != nil {
		logger.Error(err, "Failed to get krb5 path")
		return "", err
	}

	adminKeytabPath, err := k.GetAdminKeytabPath()
	defer func() {
		if err := os.RemoveAll(adminKeytabPath); err != nil {
			logger.Error(err, "Failed to remove keytab")
		}
	}()

	if err != nil {
		logger.Error(err, "Failed to get admin keytab path")
		return "", err
	}

	cmd := exec.Command("kadmin", "-kt", adminKeytabPath, "-p", *k.GetAdminPrincipal(), "-q", query)
	// https://web.mit.edu/kerberos/krb5-latest/doc/admin/install_kdc.html#edit-kdc-configuration-files
	cmd.Env = append(os.Environ(), "KRB5_CONFIG="+krb5Path)
	output, err := cmd.CombinedOutput()
	result = string(output)

	if err != nil {
		logger.Error(err, "Failed to execute kadmin query", "cmd", cmd.String(), "output", result)
		return "", err
	}
	logger.V(5).Info("executed kadmin query", "cmd", cmd.String(), "output", result)

	return result, nil
}

// Ktadd generates a keytab file for the given principals
// Usage: ktadd [-k[eytab] keytab] [-q] [-e keysaltlist] [-norandkey] [principal | -glob princ-exp] [...]
func (k *Kadmin) Ktadd(principals ...string) ([]byte, error) {
	keytab := path.Join(os.TempDir(), fmt.Sprintf("%s.keytab", uuid.New().String()))
	defer func() {
		if err := os.RemoveAll(keytab); err != nil {
			logger.Error(err, "Failed to remove keytab")
		}
	}()

	queries := []string{
		"ktadd",
		"-k",   // "-k" flag is used to specify the keytab file
		keytab, // keytab file path
		"-norandkey",
	}

	queries = append(queries, principals...)

	output, err := k.Query(strings.Join(queries, " "))
	if err != nil {
		logger.Error(err, "Failed to save keytab", "principals", principals, "keytab", keytab)
		return nil, err
	}

	logger.V(1).Info("saved keytab", "principal", principals, "keytab", keytab, "output", output)

	return os.ReadFile(keytab)
}

// AddPrincipal adds a new principal
// If a principal already exists, it kadmind not return an error.
// usage: https://web.mit.edu/kerberos/krb5-latest/doc/admin/admin_commands/kadmin_local.html#add-principal
func (k *Kadmin) AddPrincipal(principal string) error {
	// Add a mutex to avoid adding the same principal concurrently
	mutex.Lock()
	defer mutex.Unlock()
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
		logger.Error(err, "Failed to add principal", "principal", principal)
		return err
	}

	logger.V(1).Info("created a new principal", "principal", principal, "output", output)
	return nil
}
