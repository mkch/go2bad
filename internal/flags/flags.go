package flags

import (
	_ "embed"
	"flag"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/mkch/gg"
)

type Flags struct {
	Force                    bool
	ObfuscateInternalExports bool
	IncludeTests             bool
	OutDir                   string
	KeepNames                keepFlag
	Seeds                    seedsFlag
	SeedFile                 string
	Debug                    bool
	Verbose                  bool
}

type seedsFlag []string

func (f *seedsFlag) Set(value string) error {
	for _, c := range value {
		seed := strings.TrimSpace(string(c))
		if seed == "" {
			continue
		}
		*f = append(*f, seed)
	}
	return nil
}

func (f *seedsFlag) String() string {
	return strings.Join(*f, "")
}

type keepFlag struct {
	names gg.Set[string]
	pkgs  map[string]gg.Set[string]
}

// ((path_seg/)*(pkg.))?id
var reKeep = regexp.MustCompile(`^(?:((?:\w[\w\.\-_]+/)*(?:[\pL][\pL\p{Nd}]*))\.)?([\pL][\pL\p{Nd}]*)$`)

func parseKeepFlag(value string) (pkg, name string) {
	matches := reKeep.FindStringSubmatch(value)
	if matches == nil {
		return "", ""
	}
	return matches[1], matches[2]
}

func (f *keepFlag) Set(value string) error {
	for flag := range strings.SplitSeq(value, ",") {
		if err := f.setFlag(flag); err != nil {
			return err
		}
	}

	return nil
}

func (f *keepFlag) setFlag(value string) error {
	value = strings.TrimSpace(value)
	pkg, name := parseKeepFlag(value)
	if name == "" {
		return fmt.Errorf("invalid argument: %v", value)
	}

	if pkg == "" {
		if f.names == nil {
			f.names = make(gg.Set[string])
		}
		f.names.Add(name)
		return nil
	}

	if f.pkgs == nil {
		f.pkgs = make(map[string]gg.Set[string])
	}
	if names := f.pkgs[pkg]; names != nil {
		names.Add(name)
	} else {
		f.pkgs[pkg] = make(gg.Set[string])
		f.pkgs[pkg].Add(name)
	}

	return nil
}

func (f *keepFlag) Contains(pkg, name string) bool {
	if f.names != nil && f.names.Contains(name) {
		return true
	}
	if f.pkgs != nil {
		if names := f.pkgs[pkg]; names != nil {
			if names.Contains(name) {
				return true
			}
		}
		if names := f.pkgs[path.Base(pkg)]; names != nil {
			return names.Contains(name)
		}
	}

	return false
}

func (f *keepFlag) Empty() bool {
	return len(f.names) == 0 && len(f.pkgs) == 0
}

func (f *keepFlag) String() string {
	if f == nil {
		return ""
	}
	var s []string
	if f.names != nil {
		s = slices.Collect(maps.Keys(f.names))
	}
	if f.pkgs != nil {
		for pkg, names := range f.pkgs {
			for name := range names {
				s = append(s, pkg+"."+name)
			}
		}
	}
	return strings.Join(s, ",")
}

//go:embed usage.txt
var usage string

func Init() *Flags {
	var flags Flags
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), usage)
		flag.PrintDefaults()
		fmt.Fprintln(flag.CommandLine.Output(), "\nCode repository: https://github.com/mkch/go2bad")
	}
	flag.BoolVar(&flags.IncludeTests, "include-test", false, "Include tests code.")
	flag.BoolVar(&flags.IncludeTests, "t", false, "Alias for -include-test.")
	flag.BoolVar(&flags.Force, "overwrite", false, "Overwrite existing output files.")
	flag.BoolVar(&flags.Force, "f", false, "Alias for -overwrite.")
	flag.StringVar(&flags.OutDir, "out-dir", "", "Path to the output directory. Required.")
	flag.StringVar(&flags.OutDir, "o", "", "Alias for -out-dir.")
	flag.BoolVar(&flags.ObfuscateInternalExports, "obfuscate-internal-exports", false, "Obfuscate exports names in internal packages.")
	flag.BoolVar(&flags.ObfuscateInternalExports, "oie", false, "Alias for -obfuscate-internal-exports.")
	flag.Var(&flags.KeepNames, "keep", "Keep names from obfuscating. The format of name is\nName | pkg.Name | path/pkg.Name\nNames can be listed with commas or specified via repeated -keep flags.")
	flag.Var(&flags.Seeds, "seeds", "Seeds to generate obfuscated names. The characters of flag value are used as seeds. Default value is equivalent to alphanumeric.")
	flag.StringVar(&flags.SeedFile, "seed-file", "", "File contains space-separated seeds.")
	flag.BoolVar(&flags.Debug, "debug", false, "Enable debug mode.")
	flag.BoolVar(&flags.Verbose, "v", false, "Enable verbose mode.")
	flag.Parse()
	return &flags
}
