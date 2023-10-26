package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/inohime/pompk/internal/cli"
	"github.com/inohime/pompk/internal/pkgs"
	"github.com/inohime/pompk/pkg/utils"
)

func Run(flags *cli.Flags) {
	safeDoc, err := requestSafeDoc(flags)
	if err != nil {
		log.Fatalf("Failed to acquire HTML document: %v", err)
	}

	pf := pkgs.NewPipeFragment(100, 1)
	pkg := pkgs.New(flags, pf)
	swg := sync.WaitGroup{}

	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for {
				select {
				case rawPkg := <-pf.Fragments:
					swg.Add(1)
					go func(pf *pkgs.Fragment) {
						defer swg.Done()
						pkgs.Download(flags.GetOutputPath(), *pf)
					}(&rawPkg)

				case err := <-pf.QQ:
					if err != nil {
						log.Fatalln("An error has occured:", err)
					}

				case <-pf.Signal:
					return
				}
			}
		}()
	}

	pkg.Seek(safeDoc, &swg)

	swg.Wait()

	pf.Signal <- struct{}{}
}

func SetupDirectory(dirPath, pkgName string) (string, error) {
	pkgDir := filepath.Join(dirPath, pkgName)

	if _, err := os.Stat(pkgName); os.IsNotExist(err) {
		err := utils.CreateDirectory(pkgDir)
		if err != nil {
			// Fallback to the original path provided
			return dirPath, err
		}
	}

	return pkgDir, nil
}

func requestSafeDoc(flags *cli.Flags) (*utils.SafeQuery, error) {
	basePkgUrl := fmt.Sprintf(
		"https://packages.debian.org/%s/%s/%s",
		flags.GetVersion(),
		flags.GetArch(),
		flags.GetPackageName(),
	)

	doc, err := utils.RequestURL(basePkgUrl)
	if err != nil {
		return nil, err
	}

	return utils.NewSafeQuery(doc), nil
}
