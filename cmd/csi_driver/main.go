/*
Copyright 2024 zncdatadev.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/zncdatadev/secret-operator/internal/csi"
	"github.com/zncdatadev/secret-operator/internal/util/version"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	listenerv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/listeners/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	secretv1alpha1 "github.com/zncdatadev/secret-operator/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme               = runtime.NewScheme()
	setupLog             = ctrl.Log.WithName("setup")
	endpoint             = flag.String("endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	nodeID               = flag.String("nodeid", "", "node id")
	driverName           = flag.String("drivername", csi.DefaultDriverName, "name of the driver")
	probeAddr            = flag.String("health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	enableLeaderElection = flag.Bool("leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.",
	)
	versionInfo = flag.Bool("version", false, "Prints the version information")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(listenerv1alpha1.AddToScheme(scheme))

	utilruntime.Must(secretv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {

	opts := zap.Options{
		Development: true,
	}

	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if *versionInfo {
		version := version.NewAppInfo(*driverName).String()
		fmt.Println(version)
		os.Exit(0)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: *probeAddr,
		LeaderElection:         *enableLeaderElection,
		LeaderElectionID:       "8b74b19a.kubedoop.dev",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	go runMgr(ctx, mgr)

	runDriver(ctx, mgr)
}

func runMgr(ctx context.Context, mgr ctrl.Manager) {
	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func runDriver(ctx context.Context, mgr ctrl.Manager) {
	setupLog.Info("starting driver", "driver", *driverName)
	driver := csi.NewDriver(*driverName, *nodeID, *endpoint, mgr.GetClient())

	err := driver.Run(ctx, false)
	if err != nil {
		fmt.Println("Failed to run driver", "error", err.Error())
		os.Exit(1)
	}
}
