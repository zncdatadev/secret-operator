package sanity

import (
	"context"
	"os"
	"testing"

	"github.com/kubernetes-csi/csi-test/v5/pkg/sanity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zncdatadev/secret-operator/internal/csi"
)

const (
	mountPath = "/tmp/csi-mount"
	stagePath = "/tmp/csi-stage"
	socket    = "/tmp/csi.sock"
	endpoint  = "unix://" + socket
)

var (
	driver *csi.Driver
	cancel context.CancelFunc
	ctx    context.Context
)

func TestSanity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sanity Tests Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())
	driver = csi.NewDriver(
		csi.DefaultDriverName,
		"test-node",
		endpoint,
		nil,
	)
	go func() {
		Expect(driver.Run(ctx, true)).NotTo(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	cancel()
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
