package main

import (
	_ "embed"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"flag"

	"github.com/mkch/gg"
	filepath2 "github.com/mkch/gg/filepath"
	os2 "github.com/mkch/gg/os"
	slices2 "github.com/mkch/gg/slices"
	"github.com/mkch/go2bad/internal/comments"
	"github.com/mkch/go2bad/internal/flags"
	"github.com/mkch/go2bad/internal/idgen"
	"golang.org/x/tools/go/packages"
)

var cmdArgs *flags.Flags
var idGenerator *idgen.Generator

func main() {
	cmdArgs = flags.Init()
	logLevel := slog.LevelError
	if cmdArgs.Debug {
		logLevel = slog.LevelDebug
	} else if cmdArgs.Verbose {
		logLevel = slog.LevelInfo
	}
	slog.SetLogLoggerLevel(logLevel)

	slog.Debug("debug mode")

	if cmdArgs.OutDir == "" {
		slog.Error("required flag -out-dir is missing")
		os.Exit(1)
	}

	var args []string
	if args = flag.Args(); len(args) == 0 {
		args = []string{"."}
	}

	cmdArgs.KeepNames.Set("main.main")
	if len(cmdArgs.Seeds) == 0 {
		slog.Info("no seeds, use default.")
		cmdArgs.Seeds.Set("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
	}

	if cmdArgs.IncludeTests {
		slog.Info("test code will be included")
	}

	var err error
	idGenerator, err = createIDGenerator()
	if err == nil {
		err = rot(args...)
	}
	if err != nil {
		slog.Error(err.Error())
		os.Exit(2)
	}
	slog.Info("done.")
}

var reSpace = regexp.MustCompile(`\s+`)

func createIDGenerator() (*idgen.Generator, error) {
	seeds := cmdArgs.Seeds
	if cmdArgs.SeedFile != "" {
		contents, err := os.ReadFile(cmdArgs.SeedFile)
		if err != nil {
			return nil, err
		}
		seeds = append(seeds, reSpace.Split(string(contents), -1)...)
	}
	return idgen.NewGenerator(seeds...), nil
}

func internalPos(pkgPath string) int {
	// starting with path element "internal" is not an internal package
	if strings.HasSuffix(pkgPath, "/internal") {
		return len(pkgPath) - len("/internal")
	}
	return strings.LastIndex(pkgPath, "/internal/")
}

func isInternalPackage(pkgPath string) bool {
	return internalPos(pkgPath) > 0
}

func canImport(internalPkg, pkg string) bool {
	pi := internalPos(internalPkg)
	if pi <= 0 {
		panic("not an internal package")
	}
	if !strings.HasSuffix(pkg, "/") {
		pkg += "/"
	}
	parent := internalPkg[:pi+1]
	return strings.HasPrefix(pkg, parent)
}

// intrinsicImportNames returns all package identifiers introduced
// by unnamed import clauses in pkg.
func intrinsicImportNames(pkg *packages.Package) (ret gg.Set[string]) {
	ret = make(gg.Set[string])
	for _, f := range pkg.Syntax {
		type spec = *ast.ImportSpec
		imports := slices2.Filter(f.Imports, func(spec spec) bool { return spec.Name == nil || spec.Name.Name == "." })
		names := slices2.Map(imports, func(spec spec) string { return path.Base(gg.Must(strconv.Unquote(spec.Path.Value))) })
		for _, name := range names {
			ret.Add(name)
		}
	}
	return
}

func logPackageErrors(pkgs []*packages.Package) int {
	var n int
	errModules := make(map[*packages.Module]bool)
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		for _, err := range pkg.Errors {
			pos := gg.IfFunc(err.Pos == "" || err.Pos == "-",
				func() string { return err.Pos + "" },
				func() string { return "" })
			slog.Error(pos + err.Msg)
			n++
		}

		// Print pkg.Module.Error once if present.
		mod := pkg.Module
		if mod != nil && mod.Error != nil && !errModules[mod] {
			errModules[mod] = true
			slog.Error(mod.Error.Err)
			n++
		}
	})
	return n
}

func rot(pkgs ...string) (err error) {
	const mode = packages.NeedName |
		packages.NeedCompiledGoFiles |
		packages.NeedSyntax |
		packages.NeedTypesInfo |
		packages.NeedModule |
		packages.NeedEmbedFiles

	loaded, err := packages.Load(&packages.Config{
		Mode:  mode | gg.If(cmdArgs.IncludeTests, packages.NeedForTest, 0),
		Tests: cmdArgs.IncludeTests}, pkgs...)
	if err != nil {
		return
	}
	if len(loaded) == 0 {
		return errors.New("no package loaded")
	}
	if n := logPackageErrors(loaded); n > 0 {
		return fmt.Errorf("%d "+gg.If(n > 1, "errors", "error"), n)
	}

	loaded = filterPackages(loaded)

	var exportRenamers map[*packages.Package]*renamer

	// process pkg itself
	for _, pkg := range loaded {
		slog.Info("processing package...\t", "pkg", pkg.PkgPath)
		imports := intrinsicImportNames(pkg)
		defs, uses := pkg.TypesInfo.Defs, pkg.TypesInfo.Uses
		if cmdArgs.ObfuscateInternalExports && isInternalPackage(pkg.PkgPath) {
			if exportRenamers == nil {
				exportRenamers = make(map[*packages.Package]*renamer)
			}
			slog.Info("renaming exported ids...\t", "pkg", pkg.PkgPath)
			exportRenamers[pkg] = renamePackageExports(pkg.Fset, defs, uses, idGenerator.NewExported(imports))
		} else {
			slog.Debug("skipping exported id renaming...\t",
				"pkg", pkg.PkgPath,
				"internal", isInternalPackage(pkg.PkgPath),
				"-oie", cmdArgs.ObfuscateInternalExports)
		}
		slog.Info("renaming unexported ids...\t", "pkg", pkg.PkgPath)
		renamePackage(pkg.Fset, defs, uses, idGenerator.NewUnexported(imports))
	}

	// rename usage of pkg
	for internal, renamer := range exportRenamers {
		for _, pkg := range loaded {
			if pkg == internal {
				slog.Debug("skipping usage renaming...\t", "internal", internal.PkgPath, "target", pkg.PkgPath, "reason", "self")
				continue // skip pkg itself
			}

			if !canImport(internal.PkgPath, pkg.PkgPath) {
				slog.Debug("skipping usage renaming...\t", "internal", internal.PkgPath, "target", pkg.PkgPath, "reason", "can't import")
				continue
			}
			slog.Info("renaming usage...\t", "internal", internal.PkgPath, "target", pkg.PkgPath)
			for id, obj := range pkg.TypesInfo.Uses {
				renamer.Rename(id, obj)
			}
		}
	}

	// write
	for _, pkg := range loaded {
		pkgDirRel := gg.Must(filepath.Rel(gg.Must(filepath.Abs("")), pkg.Dir))
		destPkgDir := filepath.Join(cmdArgs.OutDir, pkgDirRel)
		slog.Info("writing package...\t", "pkg", pkg.PkgPath, "dest", destPkgDir)
		if err = os.MkdirAll(destPkgDir, 0777); err != nil {
			return
		}

		// go.mod and go.sum
		if mod := pkg.Module.GoMod; mod != "" {
			if pkg.Module.Dir == pkg.Dir {
				dest := filepath.Join(destPkgDir, filepath.Base(pkg.Module.GoMod))
				slog.Info("copying go.mod...\t", "from", pkg.Module.GoMod, "to", dest)
				if err = os2.CopyFile(pkg.Module.GoMod, dest, cmdArgs.Force); err != nil {
					return
				}
				sum := filepath2.ChangeExt(mod, ".sum")
				if _, statErr := os.Stat(sum); statErr == nil {
					dest = filepath.Join(destPkgDir, filepath.Base(sum))
					slog.Info("copying go.sum...\t", "from", sum, "to", dest)
					if err = os2.CopyFile(sum, dest, cmdArgs.Force); err != nil {
						return
					}
				}
			}
		}
		// go files
		for i, f := range pkg.Syntax {
			gofile := pkg.CompiledGoFiles[i]
			comments.Trim(f)
			destFilePath := filepath.Join(destPkgDir, filepath.Base(gofile))
			if err = os.MkdirAll(filepath.Dir(destFilePath), 0777); err != nil {
				return
			}
			slog.Info("writing go file...\t", "path", destFilePath)
			var w *os.File
			w, err = os.OpenFile(destFilePath, os.O_CREATE|os.O_WRONLY|gg.If(cmdArgs.Force, os.O_TRUNC, os.O_EXCL), 0666)
			if err != nil {
				return
			}
			if err = func() error {
				defer gg.ChainError(w.Close, &err)
				if err := doNotEdit(w); err != nil {
					return err
				}
				if err := format.Node(w, pkg.Fset, f); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return
			}
		}

		// other files
		for _, f := range pkg.OtherFiles {
			rel := gg.Must(filepath.Rel(pkg.Dir, f))
			dest := filepath.Join(destPkgDir, rel)
			slog.Info("copying other file...\t", "from", f, "to", dest)
			if err = os2.CopyFile(f, dest, cmdArgs.Force); err != nil {
				return
			}
		}

		// embed files
		for _, f := range pkg.EmbedFiles {
			rel := gg.Must(filepath.Rel(pkg.Dir, f))
			dest := filepath.Join(destPkgDir, rel)
			slog.Info("copying embed file...\t", "from", f, "to", dest)
			if err = os2.CopyFile(f, dest, cmdArgs.Force); err != nil {
				return
			}
		}
	}
	return nil
}

// filterPackages filter out the test binary package(pkg.test)
// and the packages whose test package presents.
func filterPackages(pkgs []*packages.Package) (result []*packages.Package) {
	if !cmdArgs.IncludeTests {
		result = pkgs
		return
	}
	result = make([]*packages.Package, 0, len(pkgs))
	var blackBoxTests []*packages.Package
	for _, pkg := range pkgs {
		if strings.HasSuffix(pkg.ID, ".test") {
			continue
		}
		// The ID of black box test package is
		// "id_pkg_under_test [id_pkg_under_test.test]"
		// The block box test package includes all files in package under test.
		testing := strings.HasSuffix(pkg.ID, ".test]")
		if testing && strings.HasPrefix(pkg.ID, pkg.ForTest+" ") {
			blackBoxTests = append(blackBoxTests, pkg)
		}
		result = append(result, pkg)
	}

	for _, black := range blackBoxTests {
		// delete the package that black is for.
		result = slices.DeleteFunc(result, func(pkg *packages.Package) bool { return pkg.ID == black.ForTest })
	}
	return
}

func doNotEdit(f *os.File) (err error) {
	// https://pkg.go.dev/cmd/go#hdr-Generate_Go_files_by_processing_source
	_, err = io.WriteString(f, "// Code generated by go2bad. DO NOT EDIT.\n\n")
	return
}

func renamePackage(fset *token.FileSet, defs, uses map[*ast.Ident]types.Object, next func() string) {
	renamer := newRenamer(fset, defs, next, func(id *ast.Ident, obj types.Object) bool {
		if id.Name == "." || id.Name == "_" || obj == nil {
			slog.Debug("skipping id...\t", "id", id.Name)
			return false
		}
		// package exports
		if isPackageExport(obj) {
			slog.Debug("skipping exported id...\t", "id", id.Name)
			return false
		}
		return true
	})
	for id, obj := range uses {
		renamer.Rename(id, obj)
	}
}

func renamePackageExports(fset *token.FileSet, defs, uses map[*ast.Ident]types.Object, next func() string) *renamer {
	renamer := newRenamer(fset, defs, next, func(id *ast.Ident, obj types.Object) bool {
		return isPackageExport(obj)
	})
	for id, obj := range uses {
		renamer.Rename(id, obj)
	}
	return renamer
}

// isPackageExport returns whether object is a package export identifier.
func isPackageExport(object types.Object) bool {
	if object == nil {
		// package names or symbolic variables
		// see https://pkg.go.dev/golang.org/go/types/#Info
		return false
	}
	scope := object.Parent()
	// only include package exports
	return object.Exported() &&
		(scope == nil || // methods or struct fields
			scope == object.Pkg().Scope()) // package level ids
}

type nameObj struct {
	newName string
	Pos     gg.Set[token.Pos]
}

type renamerFilter func(id *ast.Ident, obj types.Object) bool

type renamer struct {
	names map[string]*nameObj // key is old name
}

// TestXxx where Xxx does not start with a lowercase letter
// No id validation.
var reTestFuncName = regexp.MustCompile(`^Test[^\p{Ll}]`)

// isTestFunc returns true if obj is a test function.
func isTestFunc(fset *token.FileSet, obj types.Object) bool {
	if !strings.HasSuffix(fset.PositionFor(obj.Pos(), true).Filename, "_test.go") {
		return false
	}
	f, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	if !reTestFuncName.MatchString(f.Name()) {
		return false
	}
	signature := f.Signature()
	if signature.Recv() != nil {
		return false
	}
	params := signature.Params()
	if params == nil || signature.TypeParams() != nil || signature.Variadic() {
		return false
	}
	argumentType := types.Unalias(params.At(0).Type())
	return argumentType.String() == "*testing.T"

}

// isInitFunc returns true if obj is a package init function.
func isInitFunc(obj types.Object) bool {
	f, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	if f.Name() != "init" {
		return false
	}
	return true
}

func newRenamer(fset *token.FileSet, defs map[*ast.Ident]types.Object, next func() string, filter renamerFilter) *renamer {
	var ret = renamer{
		names: make(map[string]*nameObj),
	}

	type idObj struct {
		id  *ast.Ident
		obj types.Object
	}
	defsToRename := make(map[string][]idObj, len(defs)/2)
	keptNames := make(gg.Set[string])

	for id, obj := range defs {
		if !filter(id, obj) {
			continue
		}
		// init func
		if isInitFunc(obj) {
			keptNames.Add(id.Name)
			continue
		}
		// test func
		if isTestFunc(fset, obj) {
			keptNames.Add(id.Name)
			continue
		}
		// keep
		if cmdArgs.KeepNames.Contains(obj.Pkg().Path(), id.Name) {
			keptNames.Add(id.Name)
			continue
		}
		if kvs := defsToRename[id.Name]; kvs == nil {
			defsToRename[id.Name] = []idObj{{id, obj}}
		} else {
			defsToRename[id.Name] = append(kvs, idObj{id, obj})
		}
	}

	if len(defsToRename) == 0 {
		return &ret
	}

	for len(defsToRename) > 0 {
		newName := next()
		if keptNames.Contains(newName) {
			continue // they are kept and not used for naming to avoid name conflict.
		}
		if defsToRename[newName] != nil {
			// newName is used by the source
			delete(defsToRename, newName) // already "renamed" to newName
			continue                      // do not use it for future naming
		}
		var name string
		var kvs []idObj
		for name, kvs = range defsToRename {
			break // get a random k-v from map
		}

		objects := make(gg.Set[token.Pos])
		for _, kv := range kvs {
			objects.Add(kv.obj.Pos())
			kv.id.Name = newName
		}
		ret.names[name] = &nameObj{newName, objects}
		delete(defsToRename, name) // already renamed
	}

	return &ret
}

// Rename renames id to a rotten name.
func (rm *renamer) Rename(id *ast.Ident, obj types.Object) {
	if named, ok := rm.names[id.Name]; !ok {
		return
	} else {
		if _, in := named.Pos[obj.Pos()]; in {
			slog.Debug("renaming id usage...\t", "id", id.Name, "new", named.newName)
			id.Name = named.newName
		}
	}
}
