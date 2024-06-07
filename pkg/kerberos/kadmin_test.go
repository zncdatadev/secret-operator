package kerberos

import (
	"os/exec"
	"strings"
	"testing"
)

func TestHash(t *testing.T) {
	// subCmd := []string{"addprinc", "-randkey", "f1/localhost"}
	subCmd := []string{"ktadd", "-k", "/tmp/keytab-615459352", "-norandkey", "HTTP/nginx.default@WHG.CN"}
	cmd := exec.Command("kadmin", "-kt", "/tmp/admin-keytab-504fd4d020ec5a349c4abb4.keytab", "-p", "admin/admin", "-q", strings.Join(subCmd, " "))
	cmd.Env = append(cmd.Env, "KRB5_CONFIG=/tmp/krb5-fdf84f078a15fa94913aee8.conf")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Log(string(output))
		t.Errorf("Failed to execute kadmin query: %v", err)
	}
	t.Logf("Executed kadmin query: %v", string(output))
}
