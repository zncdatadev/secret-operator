package kreberos

import (
	"crypto/sha256"
	"os"
	"strings"
)

/*
Example krb5.conf file:

# [logging]
#  default = FILE:/tmp/krb5libs.log
#  kdc = FILE:/tmp/krb5kdc.log
#  admin_server = FILE:/tmp/kadmind.log

[libdefaults]
 default_realm = EXAMPLE.COM
#  dns_lookup_realm = false
#  dns_lookup_kdc = true
#  rdns = false
#  ticket_lifetime = 24h
#  forwardable = true
#  udp_preference_limit = 0

[realms]
 EXAMPLE.COM = {
  kdc = fodera.example.cn:88
  # master_kdc = fodera.example.cn:88
  # kpasswd_server = fodera.example.cn:464
  admin_server = fodera.example.cn:749
  # default_domain = example.cn
}

# [domain_realm]
#  .example.cn = EXAMPLE.COM
#  example.cn = EXAMPLE.COM

*/

type Krb5Config struct {
	Realm       string
	AdminServer string
	KDC         string

	hashed string
}

func (c *Krb5Config) GetRealm() string {
	return strings.ToUpper(c.Realm)
}

// ref: https://web.mit.edu/kerberos/krb5-latest/doc/admin/install_clients.html#client-machine-configuration-files
func (c *Krb5Config) content() string {
	content := `
	[libdefaults]
		default_realm = ` + c.GetRealm() + `
	
	[realms]
		` + c.GetRealm() + ` = {
			kdc = ` + c.KDC + `
			admin_server = ` + c.AdminServer + `
	`
	return content
}

// Save write krb5.conf file

// Default krb5.conf in Linux is /etc/krb5.conf, if you want to use custom krb5.conf file, you can set KRB5_CONFIG env.
func (c *Krb5Config) Save(path string) error {
	return os.WriteFile(path, []byte(c.content()), 0644)
}

func (c *Krb5Config) Hash() string {
	if c.hashed == "" {
		h := sha256.New()
		h.Write([]byte(c.content()))
		c.hashed = string(h.Sum(nil)[:24])
	}
	return c.hashed
}

func (c *Krb5Config) GetTempPath() string {
	absFilename := "/tmp/krb5-" + c.Hash() + ".conf"

	if _, err := os.Stat(absFilename); os.IsNotExist(err) {
		if err := c.Save(absFilename); err != nil {
			return ""
		}
	}

	return absFilename
}
