package main

import (
	"fmt"
	"os"
	"strings"
)

var usage = `Usage: atomdeb <command> [<options> ...]
Available commands:
		list
		install [latest|<version>]
`

var config = map[string]string{
	"latest":         "https://api.github.com/repos/atom/atom/releases/latest",
	"list":           "https://api.github.com/repos/atom/atom/releases",
	"getReleaseBase": "https://api.github.com/repos/atom/atom/releases/",
	"debName":        "atom-amd64.deb",
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		fmt.Printf("\n%v\n", usage)
		os.Exit(1)
	}
}

func run() error {

	if len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Println(usage)
		return nil
	}

	if len(os.Args) < 2 {
		return fmt.Errorf("command missing")
	}

	if strings.HasPrefix(os.Args[1], "-") {
		return fmt.Errorf("unknown option: %s", os.Args[1])
	}

	switch command := os.Args[1]; command {
	case "list":
		if len(os.Args) > 2 {
			return fmt.Errorf("list command takes no arguments")
		}
		return listCommand()
	case "install":
		if len(os.Args) > 3 {
			return fmt.Errorf("too many arguments to %s command", command)
		}

		version := os.Args[2]
		return installCommand(version)
	default:
		return fmt.Errorf("unknown command: %s", os.Args[1])
	}
}

func listCommand() error {

	f := func(r release) bool {
		ok := false
		for _, asset := range r.Assets {
			ok = asset.Name == "atom-amd64.deb" || ok
		}
		return ok
	}

	releases, err := findReleases(f)
	if err != nil {
		return err
	}

	for _, release := range *releases {
		fmt.Println(release.Name)
	}

	return nil
}

func installCommand(version string) error {
	var release *release

	if version == "latest" {
		var err error
		release, err = getRelase(version)
		if err != nil {
			return err
		}
	} else {
		var err error
		releases, err := findReleases()
		if err != nil {
			return err
		}
		release, err = releases.get(version)
		if err != nil {
			fmt.Printf("Unable to find the release \"%v\"\n", version)
			os.Exit(2)
		}
	}

	asset, err := release.get(config["debName"])
	if err != nil {
		return err
	}

	installed, err := installedDebVersion("atom")
	if err != nil && err != errNotInstalled {
		return err
	}
	if installed == release.Name {
		fmt.Println("Atom is already installed at version ", installed)
		os.Exit(2)
	}

	err = asset.download(config["debName"])
	if err != nil {
		return err
	}
	defer os.Remove(config["debName"])

	err = install(config["debName"])
	if err != nil {
		return err
	}

	return nil

}
