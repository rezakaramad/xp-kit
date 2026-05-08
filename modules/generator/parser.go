package generator

import (
	"fmt"

	"golang.org/x/tools/go/packages"
	"sigs.k8s.io/controller-tools/pkg/crd"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

// newCRDParser creates a ready-to-use controller-tools parser for the given package.
// It is the shared setup step used by both schema and printer-column extraction.
//
// It returns:
//   - a configured crd.Parser with all kubebuilder markers registered
//   - the loaded root packages (needed to build TypeIdents)
func newCRDParser(packagePath string) (*crd.Parser, []*loader.Package, error) {
	moduleDir, err := findModuleDir(packagePath)
	if err != nil {
		return nil, nil, fmt.Errorf("finding module dir: %w", err)
	}

	config := &packages.Config{Dir: moduleDir}
	roots, err := loader.LoadRootsWithConfig(config, packagePath)
	if err != nil {
		return nil, nil, fmt.Errorf("loading package %q: %w", packagePath, err)
	}
	if len(roots) == 0 {
		return nil, nil, fmt.Errorf("no packages found for path %q", packagePath)
	}

	registry := &markers.Registry{}
	generator := crd.Generator{}
	if err := generator.RegisterMarkers(registry); err != nil {
		return nil, nil, fmt.Errorf("registering markers: %w", err)
	}

	parser := &crd.Parser{
		Collector: &markers.Collector{Registry: registry},
		Checker:   &loader.TypeChecker{},
	}
	crd.AddKnownTypes(parser)

	for _, root := range roots {
		parser.NeedPackage(root)
	}

	return parser, roots, nil
}
