package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

var usage = `Usage: atomdeb <command> [<options> ...]
Available commands:
		list
		install [latest|<version>]
`

type flags struct {
	version string
}

type releases []*release

type release struct {
	Name   string   `json:"name"`
	Assets []*asset `json:"assets"`
}
type asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

type passThru struct {
	io.Reader
	total    int64 // Total # of bytes transferred
	length   int64
	progress float64
	name     string
	start    time.Time
}

// Read 'overrides' the underlying io.Reader's Read method.
// This is the one that will be called by io.Copy(). We simply
// use it to keep track of byte counts and then forward the call.
func (pt *passThru) Read(p []byte) (int, error) {
	if pt.start == *new(time.Time) {
		pt.start = time.Now()
	}

	n, err := pt.Reader.Read(p)
	if n > 0 {
		pt.total += int64(n)
		percentage := float64(pt.total) / float64(pt.length) * float64(100)

		speed := float64(pt.total) / time.Since(pt.start).Seconds()
		remainingTime := "-"
		if remaining, err := time.ParseDuration(fmt.Sprintf("%.0fs", float64(pt.length-pt.total)/speed)); err == nil {
			remainingTime = remaining.String()
		}

		is := fmt.Sprintf("\r\033[KGet %v %v/%v %.0f%%", pt.name, humanize.Bytes(uint64(pt.total)), humanize.Bytes(uint64(pt.length)), percentage)
		if percentage-pt.progress > 1 || percentage == 0 {
			is = is + fmt.Sprintf("\t\t\t%v/s %6s", humanize.Bytes(uint64(speed)), remainingTime)
			fmt.Fprint(os.Stderr, is)
			pt.progress = percentage
		}
	}

	return n, err
}

var errNotInstalled = fmt.Errorf("package go is not installed")

func installedDebVersion(pkg string) (string, error) {
	if _, err := exec.LookPath("dpkg-query"); err != nil {
		if e, ok := err.(*exec.Error); ok && e.Err == exec.ErrNotFound {
			// dpkg is missing. That's okay, we can still build the
			// package, even if we can't install it.
			return "", errNotInstalled
		}
	}

	env := os.Environ()
	env = setEnv(env, "LC_ALL", "C")
	env = setEnv(env, "LANG", "C")
	env = setEnv(env, "LANGUAGE", "C")
	cmd := exec.Command("dpkg-query", "-f", "${db:Status-Abbrev}${source:Version}", "-W", pkg)
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := err.Error()
		out := strings.TrimSpace(string(output))
		if strings.Contains(strings.ToLower(out), "no packages found") {
			return "", errNotInstalled
		}
		if len(out) > 0 {
			msg += ": " + out
		}
		return "", fmt.Errorf("while querying for installed go package version: %s", msg)
	}
	s := string(output)
	if !strings.HasPrefix(s, "ii ") {
		return "", errNotInstalled
	}
	return s[3:], nil
}

func setEnv(env []string, key, value string) []string {
	key = key + "="
	for i, s := range env {
		if strings.HasPrefix(s, key) {
			env[i] = key + value
			return env
		}
	}
	return append(env, key+value)
}

var url = map[string]string{
	"latest":  "https://api.github.com/repos/atom/atom/releases/latest",
	"list":    "https://api.github.com/repos/atom/atom/releases",
	"debName": "atom-amd64.deb",
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
		version := ""
		if len(os.Args) == 3 {
			version = os.Args[2]
		} else if len(os.Args) > 3 {
			return fmt.Errorf("too many arguments to %s command", command)
		}
		return installCommand(version)
	default:
		return fmt.Errorf("unknown command: %s", os.Args[1])
	}
}

func listCommand() error {
	resp, err := http.Get(url["list"])
	if err != nil {
		return err
	}

	robots, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	releases := &releases{}
	err = json.Unmarshal(robots, releases)
	if err != nil {
		return err
	}

	for _, release := range *releases {
		fmt.Println(release.Name)
	}
	return nil
}

func installCommand(version string) error {
	var (
		assets []*asset
		name   string
	)

	if version == "latest" {
		resp, err := http.Get(url["latest"])
		if err != nil {
			return err
		}

		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}

		release := &release{}
		err = json.Unmarshal(body, release)
		if err != nil {
			return err
		}
		assets = release.Assets
		name = release.Name
	} else {
		resp, err := http.Get(url["list"])
		if err != nil {
			return err
		}

		robots, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		releases := &releases{}
		err = json.Unmarshal(robots, releases)
		if err != nil {
			return err
		}

		for _, release := range *releases {
			if release.Name == version {
				assets = release.Assets
				name = release.Name
			}
		}
		if name == "" {
			fmt.Printf("Unable to find the release \"%v\"\n", version)
			os.Exit(2)
		}
	}

	for _, asset := range assets {
		if asset.Name == url["debName"] {

			installed, err := installedDebVersion("atom")
			if err != nil && err != errNotInstalled {
				return err
			}
			if installed == name {
				fmt.Println("Atom is already installed at version ", installed)
				os.Exit(2)
			}

			out, err := os.Create(url["debName"])
			if err != nil {
				return err
			}
			defer os.Remove(url["debName"])
			defer out.Close()

			/*
				Connecting to <domain> (<ip>)
				Get <target> <downloaded>/<total> <percent>				<speed> <time remaining>
				Fetched 7.939 kB in 5s (1.420 kB/s)
			*/

			fmt.Printf("Connecting to github.com")
			resp, err := http.Get(asset.DownloadURL)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			metered := &passThru{Reader: resp.Body, length: resp.ContentLength, name: url["debName"]}

			fmt.Printf("\rGet %v 0B/%v", url["debName"], humanize.Bytes(uint64(resp.ContentLength)))
			_, err = io.Copy(out, metered)
			if err != nil {
				return err
			}
			// Make sure we get a new line after the metered copy
			fmt.Println("")

			args := []string{"dpkg", "-i", url["debName"]}
			if os.Getuid() != 0 {
				args = append([]string{"sudo"}, args...)
			}
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("while installing atom package: %v", err)
			}
		}
	}
	return nil
}
