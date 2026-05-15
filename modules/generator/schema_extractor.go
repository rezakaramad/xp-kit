package generator

// This module takes a Go package path and struct name as input
// E.g., "github.com/rezakaramad/crossplane-toolkit/types/tenant" and "Tenant"
// It uses controller-tools to parse the Go package,
// extract the struct definition and kubebuilder markers,
// and produces the OpenAPI v3 schema and additionalPrinterColumns in a single parser pass.
// Without this module, users would have to manually write the OpenAPI schema for their XRDs, which is error-prone and hard to maintain.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-tools/pkg/crd"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

// newCRDParser creates a ready-to-use controller-tools parser for the given package.
// It returns a configured crd.Parser with all kubebuilder markers registered,
// and the loaded root packages (needed to build TypeIdents).
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

// TypeInfo holds the results of a single parser pass over a Go type:
// the OpenAPI schema and the additionalPrinterColumns.
type TypeInfo struct {
	Schema         *extv1.JSONSchemaProps
	PrinterColumns []extv1.CustomResourceColumnDefinition
}

// ExtractTypeInfo loads the Go package once and extracts both the OpenAPI schema
// and the additionalPrinterColumns for the specified type in a single parser pass.
func ExtractTypeInfo(packagePath, group, typeName, version string) (*TypeInfo, error) {
	parser, roots, err := newCRDParser(packagePath)
	if err != nil {
		return nil, err
	}

	// This creates a type identifier for the specific struct we want to generate the schema for.
	// E.g., "github.com/rezakaramad/crossplane-toolkit/modules/generator/testdata/xsimple" and "XSimple"
	// typeIdent := crd.TypeIdent{
	//     Package: <the loaded package for github.com/rezakaramad/crossplane-toolkit/modules/generator/testdata/xsimple>,
	//     Name:    "XSimple",
	// }
	typeIdent := crd.TypeIdent{Package: roots[0], Name: typeName}
	// Generate the schema for this specific type
	parser.NeedSchemaFor(typeIdent)

	// Check if the type was found and the schema was generated.
	if _, ok := parser.Schemata[typeIdent]; !ok {
		// Check every loaded root package and print any errors we already know about.
		// Gives more context when debugging.
		for _, root := range roots {
			for _, e := range root.Errors {
				fmt.Fprintf(os.Stderr, "package error: %v\n", e)
			}
		}
		return nil, fmt.Errorf("type %q not found in package %q", typeName, packagePath)
	}

	// The parser generates a "flattened" schema that resolves all references and embeds.
	parser.NeedFlattenedSchemaFor(typeIdent)

	// Check if the flattened schema is available for the type.
	schema, ok := parser.FlattenedSchemata[typeIdent]
	if !ok {
		return nil, fmt.Errorf("flattened schema not found for type %q", typeName)
	}

	// Printer columns extraction — reuse the same parser, no second package load.
	var printerColumns []extv1.CustomResourceColumnDefinition
	groupKind := k8sschema.GroupKind{Group: group, Kind: typeName}
	parser.NeedCRDFor(groupKind, nil)
	if crdObj, ok := parser.CustomResourceDefinitions[groupKind]; ok {
		for _, ver := range crdObj.Spec.Versions {
			if ver.Name == version {
				printerColumns = ver.AdditionalPrinterColumns
				break
			}
		}
		if printerColumns == nil && len(crdObj.Spec.Versions) == 1 {
			printerColumns = crdObj.Spec.Versions[0].AdditionalPrinterColumns
		}
	}

	return &TypeInfo{Schema: &schema, PrinterColumns: printerColumns}, nil
}

// Figures out where the package lives on disk
// Example input: "github.com/rezakaramad/crossplane-toolkit/modules/generator/testdata/xsimple"
// Example output: "/home/reza/projects/crossplane-toolkit/modules/generator/testdata/xsimple"
//
// Go packages can come from different places:
// 1. Your current module (e.g. "github.com/rezakaramad/crossplane-toolkit/modules/generator/testdata/xsimple")
// 2. A dependency module (e.g. "sigs.k8s.io/controller-tools")
// 3. A replace directive in go.mod that points to a local path (e.g. "github.com/rezakaramad/some-lib" → "../some-lib")
// 4. A dependency module in the Go module cache (e.g. "github.com/rezakaramad/some-lib" → "$GOPATH/pkg/mod/github.com/rezakaramad/some-lib@v1.2.3")
// So the function tries 4 strategies in order.
func findModuleDir(packagePath string) (string, error) {
	// Gives info about the currently running Go binary
	buildInfo, ok := debug.ReadBuildInfo()
	if ok {
		// Strategy 1: package belongs to the same module as the currently running program.
		// .Main.Path ==> the path of current module
		// E.g., buildInfo.Main.Path = "github.com/rezakaramad/crossplane-toolkit/modules/generator"
		//		packagePath = "github.com/rezakaramad/crossplane-toolkit/modules/generator/testdata/xsimple"
		// 		subPath = /testdata/xsimple
		//		module root = /home/kara/github/r-karamad/crossplane-toolkit/modules/generator
		// 		final path = /home/kara/github/r-karamad/crossplane-toolkit/modules/generator/testdata/xsimple

		if buildInfo.Main.Path != "" && moduleContainsPackage(buildInfo.Main.Path, packagePath) {
			root, err := findModuleRoot()
			if err != nil {
				return "", fmt.Errorf("locating module root for main module: %w", err)
			}
			subPath := strings.TrimPrefix(packagePath, buildInfo.Main.Path)
			return filepath.Join(root, subPath), nil
		}

		// Strategy 2: package is inside one of the modules the current program depends on.
		for _, dependencyModules := range buildInfo.Deps {
			// If this dependency module is nil, or if the package does not belong
			// to this dependency module, skip it and check the next one
			if dependencyModules == nil || !moduleContainsPackage(dependencyModules.Path, packagePath) {
				continue
			}
			// A replace directive in go.mod tells Go to use a local path
			// instead of normal location/version of this module.
			// E.g., "github.com/rezakaramad/some-lib" → "../some-lib"
			if dependencyModules.Replace != nil {
				// The replace path is the new location of the module on disk.
				subPath := strings.TrimPrefix(packagePath, dependencyModules.Path)
				// If the replace path is relative, resolve it to an absolute path.
				// An absolute path starts from the root of the filesystem
				// E.g., "../some-lib" → "/home/reza/some-lib"
				absPath, err := filepath.Abs(dependencyModules.Replace.Path)
				if err != nil {
					return "", fmt.Errorf("resolving replace path: %w", err)
				}
				return filepath.Join(absPath, subPath), nil
			}
			// If the dependency has a real module version
			if dependencyModules.Version != "" && dependencyModules.Version != "(devel)" {
				// Builds the expected location of that module inside the Go module cache.
				// The Go module cache usually stores downloaded modules in a directory like:
				// $GOMODCACHE/<module>@<version>
				// So if:
				// 	goModCache() = "/home/reza/go/pkg/mod"
				// 	dependencyModules.Path = "github.com/rezakaramad/some-lib"
				// 	dependencyModules.Version = "v1.2.3"
				// Then the expected path is:
				// "/home/reza/go/pkg/mod/github.com/rezakaramad/some-lib@v1.2.3"

				return filepath.Join(goModCache(), dependencyModules.Path+"@"+dependencyModules.Version), nil
			}
		}
	}

	// Strategy 2: Asks the running binary what dependencies do you know about
	// Strategy 3: Asks the go.mod file of the current module if there are any replace directives for this package
	gomodPath, err := goModFile()
	if err != nil {
		return "", fmt.Errorf("locating go.mod: %w", err)
	}

	gomodBytes, err := os.ReadFile(gomodPath)
	if err != nil {
		return "", fmt.Errorf("reading go.mod: %w", err)
	}

	mf, err := modfile.Parse(gomodPath, gomodBytes, nil)
	if err != nil {
		return "", fmt.Errorf("parsing go.mod: %w", err)
	}

	// Replace directive paths are relative to the go.mod file, not CWD.
	gomodDir := filepath.Dir(gomodPath)
	for _, r := range mf.Replace {
		if !moduleContainsPackage(r.Old.Path, packagePath) {
			continue
		}
		subPath := strings.TrimPrefix(packagePath, r.Old.Path)
		absPath := r.New.Path
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(gomodDir, absPath)
		}
		return filepath.Join(absPath, subPath), nil
	}

	// Strategy 4: fall back to module cache via require directives.
	for _, r := range mf.Require {
		if !moduleContainsPackage(r.Mod.Path, packagePath) {
			continue
		}
		return filepath.Join(goModCache(), r.Mod.Path+"@"+r.Mod.Version), nil
	}

	// Strategy 5: walk the go.work workspace (if active) and match against the
	// module declared in each local 'use' directory.
	// This covers the case where the target package lives in a workspace module
	// that is not a declared dependency of the binary being run (e.g. gen-xrd
	// receiving a --package path that belongs to types/* which it never imports).
	if workFile := goWorkFile(); workFile != "" {
		workBytes, err := os.ReadFile(workFile)
		if err == nil {
			wf, err := modfile.ParseWork(workFile, workBytes, nil)
			if err == nil {
				workDir := filepath.Dir(workFile)
				for _, use := range wf.Use {
					useDir := use.Path
					if !filepath.IsAbs(useDir) {
						useDir = filepath.Join(workDir, useDir)
					}
					useGoMod := filepath.Join(useDir, "go.mod")
					useGoModBytes, err := os.ReadFile(useGoMod)
					if err != nil {
						continue
					}
					useMf, err := modfile.Parse(useGoMod, useGoModBytes, nil)
					if err != nil || useMf.Module == nil {
						continue
					}
					if !moduleContainsPackage(useMf.Module.Mod.Path, packagePath) {
						continue
					}
					subPath := strings.TrimPrefix(packagePath, useMf.Module.Mod.Path)
					return filepath.Join(useDir, subPath), nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not find module directory for package %q", packagePath)
}

// findModuleRoot walks up from the current working directory to find the
// directory containing go.mod, which is the root of the Go module.
func findModuleRoot() (string, error) {
	// Start from the current working directory
	// E.g., "/home/reza/projects/crossplane-toolkit/modules/generator"
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	// Loop until we find a go.mod file or reach the root of the filesystem
	for {
		// If go.mod exists in this directory, we found the module root
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		// Get the parent directory
		parent := filepath.Dir(dir)
		// If the parent is the same as the current directory,
		// we have reached the root of the filesystem without finding go.mod
		if parent == dir {
			return "", fmt.Errorf("go.mod not found in any parent directory")
		}
		// Otherwise, move up one level and check again
		dir = parent
	}
}

// Finds the Go module cache directory
func goModCache() string {
	// GOMODCACHE environment variable allows users to specify a custom location for the Go module cache.
	if gomodcache := os.Getenv("GOMODCACHE"); gomodcache != "" {
		return gomodcache
	}

	// If GOMODCACHE is not set, we can ask Go where the module cache is by running "go env GOMODCACHE".
	cacheDir, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err == nil {
		return strings.TrimSpace(string(cacheDir))
	}

	// '~/go/pkg/mod' is the default standard location of the Go module cache
	// if none of GOMODCACHE environment variable or "go env GOMODCACHE" command provide a valid path.
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "go", "pkg", "mod")
}

// Finds the active go.mod file for the current module
func goModFile() (string, error) {
	// Which go.mod file it needs to read
	// can be determined by asking Go directly via "go env GOMOD"
	goModOutput, err := exec.Command("go", "env", "GOMOD").Output()
	if err == nil {
		goModPath := strings.TrimSpace(string(goModOutput))
		if goModPath != "" && goModPath != os.DevNull {
			return goModPath, nil
		}
	}
	// If "go env GOMOD" fails or returns an invalid path,
	// we can find the module root and look for go.mod there.
	root, err := findModuleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "go.mod"), nil
}

// goWorkFile returns the path to the active go.work file, or "" if none is active.
func goWorkFile() string {
	out, err := exec.Command("go", "env", "GOWORK").Output()
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(string(out))
	if p == "" || p == os.DevNull {
		return ""
	}
	return p
}

// Checks if a package really belongs to a module
func moduleContainsPackage(modulePath, packagePath string) bool {
	return packagePath == modulePath || strings.HasPrefix(packagePath, modulePath+"/")
}
