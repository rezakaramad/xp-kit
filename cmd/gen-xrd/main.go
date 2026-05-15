// gen-xrd generates a Crossplane CompositeResourceDefinition YAML from an
// annotated Go struct. It reads kubebuilder validation markers from the source
// and emits a fully populated XRD ready to apply to a cluster.
//
// Usage:
//
//	gen-xrd --package <import-path> --type <TypeName> --group <api-group> \
//	        [--version <version>] [--plural <plural>] [--output <file>]
//
// Example:
//
//	gen-xrd \
//	  --package github.com/rezakaramad/crossplane-toolkit/types/xtenant \
//	  --type    XTenant \
//	  --group   idp.rezakara.demo \
//	  --version v1beta1 \
//	  --plural  xtenants \
//	  --output  xtenant-xrd.yaml
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/rezakaramad/crossplane-toolkit/modules/generator"
)

func main() {
	pkg := flag.String("package", "", "Go import path of the package containing the XR type (required)")
	typeName := flag.String("type", "", "Go struct name of the composite resource (required)")
	group := flag.String("group", "", "Crossplane API group (required), e.g. idp.rezakara.demo")
	version := flag.String("version", "v1alpha1", "API version (default: v1alpha1)")
	plural := flag.String("plural", "", "Plural resource name; defaults to lowercase kind + 's'")
	scope := flag.String("scope", "Namespaced", "XRD scope: Namespaced or Cluster")
	defaultComposition := flag.String("default-composition", "", "Name of the default Composition (sets defaultCompositionRef)")
	output := flag.String("output", "", "Write YAML to this file (default: stdout)")
	flag.Parse()

	if *pkg == "" || *typeName == "" || *group == "" {
		fmt.Fprintln(os.Stderr, "error: --package, --type, and --group are required")
		fmt.Fprintln(os.Stderr)
		flag.Usage()
		os.Exit(1)
	}

	xrd, err := generator.BuildCompositeResourceDefinition(generator.ResourceMeta{
		PackagePath:           *pkg,
		TypeName:              *typeName,
		Group:                 *group,
		Version:               *version,
		Plural:                *plural,
		Scope:                 *scope,
		DefaultCompositionRef: *defaultComposition,
	})
	if err != nil {
		log.Fatalf("generate XRD: %v", err)
	}

	out, err := generator.MarshalXRDToYAML(xrd)
	if err != nil {
		log.Fatalf("marshal XRD to YAML: %v", err)
	}

	if *output != "" {
		if err := os.WriteFile(*output, out, 0o644); err != nil {
			log.Fatalf("write %s: %v", *output, err)
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", *output)
		return
	}

	os.Stdout.Write(out)
}
