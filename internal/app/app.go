package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	Name        = "onesearch"
	Description = "CLI-first multi-source web research for AI agents and terminal users."
)

var (
	BuildVersion string
	Version      = resolveVersion()
	UserAgent    = "onesearch/" + Version
)

func resolveVersion() string {
	if version := strings.TrimSpace(BuildVersion); version != "" {
		return version
	}
	for _, path := range versionFileCandidates() {
		if version := readVersionFile(path); version != "" {
			return version
		}
	}
	return "dev"
}

func versionFileCandidates() []string {
	var candidates []string
	if exe, err := os.Executable(); err == nil && exe != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "version"))
	}

	_, sourceFile, _, ok := runtime.Caller(0)
	if ok && filepath.IsAbs(sourceFile) {
		candidates = append(candidates, filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", ".deploy", "version")))
	}
	return candidates
}

func readVersionFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
