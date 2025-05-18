package main

import (
	_ "embed"
	"errors"
	"fmt"
	"go/format"
	"go/token"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"flag"

	"github.com/mkch/gg"
	filepath2 "github.com/mkch/gg/filepath"
	"github.com/mkch/gg/os2"
	"github.com/mkch/go2bad/internal/comments"
	"github.com/mkch/go2bad/internal/flags"
	"github.com/mkch/go2bad/internal/idgen"
	"github.com/mkch/go2bad/internal/renamer"
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
		err = rename(args...)
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

func rename(pkgs ...string) (err error) {
	const mode = packages.NeedTypes |
		packages.NeedName |
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

	var renamedExports map[token.Pos]string
	for _, pkg := range loaded {
		renameExported := isInternalPackage(pkg.PkgPath) && cmdArgs.RenameInternalExports
		if renameExported {
			renamedExports = make(map[token.Pos]string)
		}
		renamer.Rename(pkg, idGenerator, renameExported, renamedExports, cmdArgs.KeepNames.Contains)
	}

	for _, pkg := range loaded {
		renamer.RenameUsedExports(pkg, renamedExports)
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
