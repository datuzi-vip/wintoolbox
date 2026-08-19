package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tc-hib/winres"
)

func main() {
	var (
		manifestPath string
		iconPath     string
		outPath      string
		arch         string
	)

	flag.StringVar(&manifestPath, "manifest", "app.manifest", "path to Windows manifest xml")
	flag.StringVar(&iconPath, "ico", "assets/app.ico", "path to application icon .ico")
	flag.StringVar(&outPath, "o", "rsrc.syso", "output COFF/rsrc syso file")
	flag.StringVar(&arch, "arch", "amd64", "target architecture: amd64|386|arm|arm64")
	flag.Parse()

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		panic(err)
	}

	appManifest, err := winres.AppManifestFromXML(manifestData)
	if err != nil {
		panic(err)
	}

	f, err := os.Open(iconPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	ico, err := winres.LoadICO(f)
	if err != nil {
		panic(err)
	}

	rs := winres.ResourceSet{}
	// Wails expects window icon resource ID = 3 (winc.AppIconID == 3),
	// so we set resID = winres.RT_ICON (which equals 3).
	if err := rs.SetIcon(winres.RT_ICON, ico); err != nil {
		panic(err)
	}
	// Embed manifest as RT_MANIFEST.
	rs.SetManifest(appManifest)

	out, err := os.Create(outPath)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	var targetArch winres.Arch
	switch arch {
	case "amd64":
		targetArch = winres.ArchAMD64
	case "386":
		targetArch = winres.ArchI386
	case "arm":
		targetArch = winres.ArchARM
	case "arm64":
		targetArch = winres.ArchARM64
	default:
		panic(fmt.Sprintf("unsupported arch: %s", arch))
	}

	if err := rs.WriteObject(out, targetArch); err != nil {
		panic(err)
	}

	fmt.Printf("wrote %s\n", outPath)
}
