// Package main implements a Crossplane Composition Function that validates
// and gates XTenant resources before provisioning begins.
package main

import (
	"github.com/alecthomas/kong"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/crossplane/function-sdk-go"
)

// CLI of this Function.
type CLI struct {
	Debug              bool   `help:"Emit debug logs in addition to info logs."                                                     short:"d"`
	Network            string `default:"tcp"                                                                                        help:"Network on which to listen for gRPC connections."`
	Address            string `default:":9443"                                                                                      help:"Address at which to listen for gRPC connections."`
	TLSCertsDir        string `env:"TLS_SERVER_CERTS_DIR"                                                                           help:"Directory containing server certs (tls.key, tls.crt) and the CA used to verify client certificates (ca.crt)"`
	Insecure           bool   `help:"Run without mTLS credentials. If you supply this flag --tls-server-certs-dir will be ignored."`
	MaxRecvMessageSize int    `default:"4"                                                                                          help:"Maximum size of received messages in MB."`
}

// Run this Function.
func (c *CLI) Run() error {
	log, err := function.NewLogger(c.Debug)
	if err != nil {
		return err
	}

	// Build a Kubernetes client so validators can query cluster state if needed.
	cfg, err := ctrlconfig.GetConfig()
	if err != nil {
		return err
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	kubeClient, err := ctrlclient.New(cfg, ctrlclient.Options{Scheme: scheme})
	if err != nil {
		return err
	}

	fn := &Function{
		log:  log,
		kube: kubeClient,
		// dns is intentionally nil — buildDNSClient selects the provider at
		// request time using input.DNS.provider from the Composition step config.
	}

	return function.Serve(fn,
		function.Listen(c.Network, c.Address),
		function.MTLSCertificates(c.TLSCertsDir),
		function.Insecure(c.Insecure),
		function.MaxRecvMessageSize(c.MaxRecvMessageSize*1024*1024))
}

func main() {
	ctx := kong.Parse(&CLI{}, kong.Description("A Crossplane Composition Function."))
	ctx.FatalIfErrorf(ctx.Run())
}
