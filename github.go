package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/dustin/go-humanize"
)

/*
	list releases
	find release
	find asset
	download asset
*/

type releases []*release

func (r *releases) get(version string) (*release, error) {
	for _, release := range *r {
		if release.Name == version {
			return release, nil
		}
	}

	return nil, fmt.Errorf("Unable to find the release \"%v\"\n", version)
}

type release struct {
	Name   string   `json:"name"`
	Assets []*asset `json:"assets"`
}

func getRelase(id string) (*release, error) {
	resp, err := http.Get(config["getReleaseBase"] + id)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	release := &release{}
	err = json.Unmarshal(body, release)
	if err != nil {
		return nil, err
	}

	return release, nil
}

func (r *release) get(name string) (*asset, error) {
	for _, asset := range r.Assets {
		if asset.Name == config["debName"] {
			return asset, nil
		}
	}
	return nil, fmt.Errorf("Unable to find the asset \"%v\"\n", name)
}

type asset struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	DownloadURL string `json:"browser_download_url"`
}

func (a *asset) download(name string) error {
	out, err := os.Create(name)
	if err != nil {
		return err
	}
	defer out.Close()

	// media type "application/octet-stream"
	fmt.Printf("Connecting to github.com")
	client := &http.Client{}
	req, _ := http.NewRequest("GET", a.URL, nil)
	req.Header.Set("Accept", "application/octet-stream")
	resp, _ := client.Do(req)

	defer resp.Body.Close()

	metered := &meteredReader{Reader: resp.Body, length: resp.ContentLength, name: a.Name}

	fmt.Printf("\rGet %v 0B/%v", a.Name, humanize.Bytes(uint64(resp.ContentLength)))
	_, err = io.Copy(out, metered)
	if err != nil {
		return err
	}
	// Make sure we get a new line after the metered copy
	fmt.Println("")

	return nil
}

func findReleases(filters ...func(release) bool) (*releases, error) {
	result := releases{}

	resp, err := http.Get(config["list"])
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	releases := &releases{}
	err = json.Unmarshal(body, releases)
	if err != nil {
		return nil, err
	}

	for _, relase := range *releases {
		keep := true
		for _, f := range filters {
			keep = f(*relase) && keep
		}
		if keep {
			result = append(result, relase)
		}
	}

	return &result, nil
}
