package pkgs

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/inohime/pompk/internal/cli"
	"github.com/inohime/pompk/pkg/utils"
)

// Not really a great idea to use global vars
// but flags can only be parsed once
// and this is quick solution
var gFlags *cli.Flags

func TestSeek(t *testing.T) {
	dir := t.TempDir()
	pkgURL := "https://packages.debian.org/bookworm/amd64/libdbus-1-3"

	doc, err := utils.RequestURL(pkgURL)
	if err != nil {
		t.Fatal("Failed to request HTML document:", err)
	}

	flags := cli.ParseFlags(dir)
	gFlags = flags
	pf := NewPipeFragment(100, 1)
	pkg := New(flags, pf)
	swg := sync.WaitGroup{}
	var qq error = nil

	go func() {
		for {
			select {
			case rawPkg := <-pf.Fragments:
				swg.Add(1)
				go func(pf *Fragment) {
					defer swg.Done()
					Download(flags.GetOutputPath(), *pf)
				}(&rawPkg)

			case err := <-pf.QQ:
				if err != nil {
					qq = err
				}

			case <-pf.Signal:
				return
			}
		}
	}()

	pkg.Seek(utils.NewSafeQuery(doc), &swg)

	swg.Wait()

	pf.Signal <- struct{}{}

	if qq != nil {
		t.Fatal("An error has occured:", qq)
	}

	f, err := os.Open(dir)
	if err != nil {
		t.Fatal("Failed to open file:", err)
	}
	defer f.Close()

	// Verify that the folder is not empty
	_, err = f.Readdirnames(1)
	if err == io.EOF {
		t.Fatal("No files were found:", err)
	}

	// These files with version tags may change in the future
	// Regex pattern or an alternative may be desirable
	files := []string{
		"libdbus-1-3_1.14.10-1~deb12u1_amd64.deb",
		"libsystemd0_252.17-1~deb12u1_amd64.deb",
		"libcap2_2.66-4_amd64.deb",
		"libgcrypt20_1.10.1-3_amd64.deb",
		"libgpg-error0_1.46-1_amd64.deb",
		"liblz4-1_1.9.4-1_amd64.deb",
		"liblzma5_5.4.1-0.2_amd64.deb",
		"libzstd1_1.5.4+dfsg2-5_amd64.deb",
	}

	// Check if all packages were downloaded
	for _, file := range files {
		filePath := fmt.Sprintf("%s/%s", dir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Fatalf("File does not exist: %v, found: %s", err, "")
		}
		t.Log("found file:", file)
	}
}

func TestSeekDownloadURL(t *testing.T) {
	dir := t.TempDir()
	pkgURL := "https://packages.debian.org/bookworm/amd64/libdbus-1-3"

	doc, err := utils.RequestURL(pkgURL)
	if err != nil {
		t.Fatal("Failed to request HTML document:", err)
	}

	flags := gFlags
	if flags == nil {
		flags = cli.ParseFlags(dir)
	} else {
		flags.SetOutputPath(dir)
	}

	dq := make(chan string, 1)
	pf := NewPipeFragment(0, 0)
	pkg := New(flags, pf)

	go pkg.seekDownloadURL(utils.NewSafeQuery(doc))

	select {
	case pkg := <-pf.Fragments:
		t.Logf("Received package fragment: %s\n", pkg.URL)
		dq <- pkg.URL

	case err := <-pf.QQ:
		if err != nil {
			t.Fatal("Failed to receive a package:", err)
		}
	}

	// Can't add testing for security packages because we
	// can't guarantee the page to reach the download URL will
	// be accessible in the future

	downloadURL := <-dq

	if downloadURL == "" {
		t.Fatalf("Failed to find package download URL")
	}

	// Should use regex to match outside of the specific version number
	expectedDownloadURL := "http://ftp.us.debian.org/debian/pool/main/d/dbus/libdbus-1-3_1.14.10-1~deb12u1_amd64.deb"
	if downloadURL != expectedDownloadURL {
		t.Fatalf("Expected download URL '%s'\n, received '%s'\n", expectedDownloadURL, downloadURL)
	}
}

func TestDownload(t *testing.T) {
	dir := t.TempDir()
	pf := Fragment{
		Name: "libdbus-1-3",
		URL:  "http://ftp.us.debian.org/debian/pool/main/d/dbus/libdbus-1-3_1.14.10-1~deb12u1_amd64.deb",
	}

	err := Download(dir, pf)
	if err != nil {
		t.Fatal("Failed to download package:", err)
	}

	if _, err := os.Stat(dir + "/" + path.Base(pf.URL)); os.IsNotExist(err) {
		t.Fatal("Package was not found:", err)
	}
}

func TestGetAllPackages(t *testing.T) {
	pkgURL := "https://packages.debian.org/bookworm/amd64/libsystemd0"

	doc, err := utils.RequestURL(pkgURL)
	if err != nil {
		t.Fatal("Failed to request HTML document:", err)
	}

	// getAllPackages only gets the packages on the current page, not nested pages
	packages := getAllPackages(utils.NewSafeQuery(doc), false)
	var pkgNames [6]string

	packages.Each(func(i int, s *goquery.Selection) {
		pkgNames[i] = s.Text()
	})

	expectedPkgNames := [...]string{
		"libc6",
		"libcap2",
		"libgcrypt20",
		"liblz4-1",
		"liblzma5",
		"libzstd1",
	}

	// Compare arrays
	if pkgNames != expectedPkgNames {
		b := strings.Builder{}

		for _, v := range expectedPkgNames {
			b.WriteString(v + ", ")
		}

		// Remove the last ", "
		expected := b.String()[0:strings.LastIndex(b.String(), ", ")]
		b.Reset()

		for _, v := range pkgNames {
			b.WriteString(v + ", ")
		}

		result := b.String()[0:strings.LastIndex(b.String(), ", ")]

		t.Fatalf("Expected packages: %s\n, Received: %s\n", expected, result)
	}
}

func TestGetPackageName(t *testing.T) {
	pkgURL := "https://packages.debian.org/bookworm/amd64/libsystemd0"

	doc, err := utils.RequestURL(pkgURL)
	if err != nil {
		t.Fatal("Failed to request HTML document:", err)
	}

	pkgName := getPackageName(utils.NewSafeQuery(doc))
	expectedPkgName := "libsystemd0"

	if pkgName != expectedPkgName {
		t.Fatalf(
			"Expected package name: %s, Received: %s\n",
			expectedPkgName,
			pkgName,
		)
	}
}
