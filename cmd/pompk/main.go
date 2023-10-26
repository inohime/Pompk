package main

import (
	"log"
	"os"

	"github.com/inohime/pompk/internal/app"
	"github.com/inohime/pompk/internal/cli"
)

func main() {
	currDir, err := os.Getwd()
	if err != nil {
		log.Fatalln("Failed to get working directory:", err)
	}

	flags := cli.ParseFlags(currDir)

	if flags.GetOutputPath() == currDir {
		outPath, err := app.SetupDirectory(currDir, flags.GetPackageName())
		if err != nil {
			log.Fatalln("Failed to setup directory:", err)
		}

		flags.SetOutputPath(outPath)
	}

	app.Run(flags)

	log.Println(
		"Finished downloading packages to:",
		flags.GetOutputPath(),
	)
}
