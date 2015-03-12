package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var errNotInstalled = fmt.Errorf("package go is not installed")

func install(deb string) error {
	args := []string{"dpkg", "-i", deb}
	if os.Getuid() != 0 {
		args = append([]string{"sudo"}, args...)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("while installing atom package: %v", err)
	}

	return nil
}

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
