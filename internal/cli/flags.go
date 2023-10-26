package cli

import (
	"flag"
	"runtime"
)

type Flags struct {
	pkg     *string
	version *string
	arch    *string
	mirror  *string
	path    *string
	noLibc  *bool
}

func ParseFlags(cwdPath string) *Flags {
	inPkg := flag.String("package", "wpasupplicant", "The package to download")
	inVersion := flag.String("version", "bookworm", "The Debian version to search packages for")
	inArch := flag.String("arch", runtime.GOARCH, "The CPU architecture to look for in packages")
	inMirror := flag.String("mirror", "ftp.us.debian.org/debian", "The nearest location URL to get the packages from")
	inPath := flag.String("path", cwdPath, "The path to the directory to download the packages to")
	inLibc := flag.Bool("nolibc", true, "Ignore the Libc package")

	flag.Parse()

	if *inPkg == "libc6" && *inLibc {
		// If the package is libc and nolibc flag is set,
		// we should just ignore the latter as the package takes
		// priority over the nolibc flag
		*inLibc = false
	}

	return &Flags{
		pkg:     inPkg,
		version: inVersion,
		arch:    inArch,
		mirror:  inMirror,
		path:    inPath,
		noLibc:  inLibc,
	}
}

func (f *Flags) GetPackageName() string {
	return *f.pkg
}

func (f *Flags) GetVersion() string {
	return *f.version
}

func (f *Flags) GetArch() string {
	return *f.arch
}

func (f *Flags) GetMirrorPath() string {
	return *f.mirror
}

func (f *Flags) GetOutputPath() string {
	return *f.path
}

func (f *Flags) SetOutputPath(filePath string) {
	*f.path = filePath
}

func (f *Flags) IsLibcAllowed() bool {
	return *f.noLibc
}
