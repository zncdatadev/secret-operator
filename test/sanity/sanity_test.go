package sanity

import (
	"context"
	"github.com/kubernetes-csi/csi-test/v5/pkg/sanity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zncdata-labs/secret-operator/internal/controller/csi"
	"os"
	"testing"
)

const (
	mountPath = "/tmp/csi-mount"
	stagePath = "/tmp/csi-stage"
	socket    = "/tmp/csi.sock"
	endpoint  = "unix://" + socket
)

var driver *csi.Driver

func TestSanity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sanity Tests Suite")
}

var _ = BeforeSuite(func() {
	driver = csi.NewDriver(
		csi.DefaultDriverName,
		"test-node",
		endpoint,
		nil,
	)
	go func() {
		Expect(driver.Run(context.Background(), true)).NotTo(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	driver.Stop()
	Expect(os.RemoveAll(socket)).NotTo(HaveOccurred())
	Expect(os.RemoveAll(mountPath)).NotTo(HaveOccurred())
	Expect(os.RemoveAll(stagePath)).NotTo(HaveOccurred())
})

var _ = Describe("CSI Driver", func() {
	config := sanity.NewTestConfig()
	config.Address = endpoint
	config.TargetPath = mountPath
	config.StagingPath = stagePath
	config.RemoveTargetPath = os.RemoveAll
	config.IdempotentCount = 1
	sanity.GinkgoTest(&config)
})
