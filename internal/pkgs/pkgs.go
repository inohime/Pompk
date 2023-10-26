package pkgs

import (
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/inohime/pompk/internal/cli"
	"github.com/inohime/pompk/pkg/utils"
)

const (
	// # Selector that contains the dependency types for a package
	//
	// <ul class="uldep">
	// 	...
	// 		<dt>
	// 			<span class=".nonvisual"> ... </span>
	//		</dt>
	// 	...
	// </ul>
	depTag = ".uldep dt span"

	// # Package Requirement Type
	//
	// Options to avoid: rec(ommends), sug(gestion), enh(ances)
	// as we only want dep(endencies) (packages we need)
	pkgReqType = "dep:"

	// # Selector for Package name in the download section
	//
	// <div id="pdownload">
	// 	...
	// 		<h2>Download { Package Name }</h2>
	//	...
	// </div>
	pkgNameTag = "#pdownload h2"

	// # Selector for the URL to download a package from
	//
	// (not to be confused with dlTag), used in seekDownloadURL
	//
	// <div id="content">
	//	...
	//		<li>
	//			<a href={ Package Download URL }>{ Mirror URL }</a>
	//		</li>
	//	...
	// </div>
	//
	// Recently, libssl3 updated to deb12u2, breaking
	// the mirrorTag search (there are no mirrors, only security).
	// It would be inefficient to have a separate mirrorTag based on
	// <div class="cardleft"> ... </div>
	// when all package links reside in <div id="content"> ... </div>
	// , so we generalize and only search for that.
	pkgURLTag = "#content li a"

	// # Selector for the package download URL from the package's base page
	//
	// ... // resides in #pdownload
	// 	<th>
	//		<a href={ Package Download URL }>{ ARCH }</a>
	// 	</th>
	// ...
	dlTag = "th a"
)

// Originally named PackageInfo, just contains
// some small data about the package for the
// download process
type Fragment struct {
	// Name of the package
	Name string
	// Download URL for the package
	URL string
}

// Fragment & Error channels
type PipeFragment struct {
	Fragments chan Fragment
	QQ        chan error
	Signal    chan struct{}
}

func NewPipeFragment(fSize, eSize int) *PipeFragment {
	return &PipeFragment{
		Fragments: make(chan Fragment, fSize),
		QQ:        make(chan error, eSize),
		Signal:    make(chan struct{}, 1),
	}
}

func (q *PipeFragment) sendFragment(pf Fragment) {
	q.Fragments <- pf
}

func (q *PipeFragment) sendError(err error) {
	q.QQ <- err
}

type PackageContext struct {
	Shortcake *utils.ShortCache
	Flags     *cli.Flags
	Castella  *PipeFragment
}

func New(flags *cli.Flags, q *PipeFragment) *PackageContext {
	return &PackageContext{
		Shortcake: utils.NewCache(),
		Flags:     flags,
		Castella:  q,
	}
}

// Seek(Package)
//
// Searches for the main package and all of its dependencies
func (pc *PackageContext) Seek(safeDoc *utils.SafeQuery, swg *sync.WaitGroup) {
	pkgName := getPackageName(safeDoc)

	if exists := pc.Shortcake.Get(pkgName); exists {
		// Avoid doing extra work
		return
	}

	pc.Shortcake.Add(pkgName)

	go pc.seekDownloadURL(safeDoc)

	packages := getAllPackages(safeDoc, pc.Flags.IsLibcAllowed())
	// In the event there are no more dependencies, stop
	if len(packages.Nodes) == 0 {
		return
	}

	packages.Each(func(_ int, s *goquery.Selection) {
		swg.Add(1)
		go func() {
			defer swg.Done()

			nextPkgURL := fmt.Sprintf(
				"https://packages.debian.org/bookworm/amd64/%s",
				s.Text(),
			)

			nextDoc, err := utils.RequestURL(nextPkgURL)
			if err != nil {
				pc.Castella.sendError(err)
				return
			}

			// Create a clone so we don't modify a document for a goroutine
			// that's moving slower/faster (faster than mutex)
			safeNextDoc := utils.NewSafeQuery(goquery.CloneDocument(nextDoc))
			pc.Seek(safeNextDoc, swg)
		}()
	})
}

// Downloads packages to the specified directory
func Download(filePath string, pf Fragment) error {
	fileName := path.Base(pf.URL)
	fileOutPath := fmt.Sprintf("%s/%s", filePath, fileName)

	err := utils.WriteFile(pf.URL, fileOutPath)
	if err != nil {
		return err
	}

	return nil
}

func (pc *PackageContext) seekDownloadURL(safeDoc *utils.SafeQuery) {
	// These child elements in particular hold the url to the download URL
	safeDoc.Read().Find(dlTag).Each(func(_ int, s *goquery.Selection) {
		// Named 'partialURL' because it only contains
		// the relative URL path (missing scheme & domain)
		// Ex: /bookworm/amd64/libdbus-1-3/download
		partialURL, exists := s.Attr("href")
		// Lacks the download URL to go to
		if !exists || !strings.Contains(partialURL, pc.Flags.GetArch()) {
			return
		}

		// Parts schema: /version/instructionSet/packageName/download
		// Skip the first element since it becomes an empty space
		URLParts := strings.Split(partialURL, "/")[1:]

		// Verify that the download url is of our debian version and is a download
		if URLParts[0] != pc.Flags.GetVersion() && URLParts[3] != "download" {
			return
		}

		packageDownloadURL := fmt.Sprintf("https://packages.debian.org%s", partialURL)

		dlDoc, err := utils.RequestURL(packageDownloadURL)
		if err != nil {
			pc.Castella.sendError(err)
			return
		}

		dlDoc.Find(pkgURLTag).Each(func(_ int, inner *goquery.Selection) {
			downloadURL, exists := inner.Attr("href")
			if !exists {
				// Optionally could get the next
				// FTP (mirror url scheme) tagged child element
				// instead of giving up
				return
			}

			// Declare ahead of time so we can jump if
			// we have a security package
			var downloadURLDP []string
			var domain string
			var updatedDP string

			if strings.Contains(downloadURL, "security.debian.org/debian-security") {
				// Skip over conditional that only applies to non-security packages
				// since it would mangle this URL and miss it
				goto sendOffFragment
			}

			// 'downloadURL' Domain + Path
			// Remove scheme from the URL and collect
			downloadURLDP = strings.Split(downloadURL, "//")[1:]
			// Take only the domain from the rest of the URL, then
			// reappend debian to match mirrorURL
			domain = strings.Split(strings.Join(downloadURLDP, ""), "/debian")[0]
			updatedDP = domain + "/debian"

			if updatedDP != pc.Flags.GetMirrorPath() {
				return
			}

		sendOffFragment:
			// Add package fragment to download queue
			pc.Castella.sendFragment(Fragment{
				Name: URLParts[2],
				URL:  downloadURL,
			})
		})
	})
}

// GetAllPackages returns packages that satisfy the following process
//
//  1. Finds all elements that satisfy the depedency tag selector
//  2. Removes elements that don't match the package requirement type
//  3. Next func gets the sibling element which would be:
//
// <a href="/OS_Version">{ Package Name }</a>
//
//  4. Remove any package labeled libc6 (unless libc is allowed)
func getAllPackages(safeDoc *utils.SafeQuery, skipLibc bool) *goquery.Selection {
	return safeDoc.
		Read().
		Find(depTag).
		FilterFunction(func(_ int, s *goquery.Selection) bool {
			return s.Text() == pkgReqType
		}).
		Next().
		FilterFunction(func(_ int, s *goquery.Selection) bool {
			return !(skipLibc && strings.Contains(s.Text(), "libc6"))
		})
}

func getPackageName(safeDoc *utils.SafeQuery) string {
	dlPkgName := safeDoc.Read().Find(pkgNameTag)
	// h2 element: "Download {package name}", we want the package name from it
	return strings.Split(dlPkgName.Text(), "Download ")[1]
}
