package main

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Item struct {
	Name        string
	Time        string
	DisplayTime string
	Size        int64
}

type BuildResult struct {
	Commit string
	Items  []Item
}

const binDir = "bin"

var args = [][]string{
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
}

func getVersion() string {
	hash, err := exec.Command("git", "rev-parse", "--short=7", "HEAD").Output()
	if err != nil {
		log.Fatalf("Error getting commit: %s\n", err)
	}
	return strings.TrimSpace(string(hash))
}

func getItem(name string) Item {
	fileInfo, err := os.Stat(binDir + "/" + name)
	if err != nil {
		log.Fatalf("Error reading file: %s\n", binDir+"/"+name)
	}
	return Item{
		Name:        name,
		Time:        fileInfo.ModTime().Format(time.RFC3339),
		DisplayTime: fileInfo.ModTime().Format(time.RFC1123),
		Size:        fileInfo.Size(),
	}
}

func build() BuildResult {
	version := getVersion()
	log.Printf("Build version: %s\n", version)
	result := BuildResult{
		Commit: version,
	}
	for _, item := range args {
		buildOs := item[0]
		buildArch := item[1]
		log.Printf("Build os=%s, arch=%s\n", buildOs, buildArch)
		name := "runic-" + buildOs + "-" + buildArch
		cmd := exec.Command(
			"go",
			"build",
			"-ldflags",
			"-s -w -X main.version="+version,
			"-trimpath",
			"-o",
			binDir+"/"+name,
			"./cmd/runic",
		)
		env := os.Environ()
		cmd.Env = append(env, "CGO_ENABLED=0", "GOOS="+buildOs, "GOARCH="+buildArch)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		err := cmd.Run()
		if err != nil {
			log.Fatalf("Failed building os=%s, arch=%s\n", buildOs, buildArch)
		}
		result.Items = append(result.Items, getItem(name))
	}
	os.WriteFile(binDir+"/version.txt", []byte(version), 0644)
	log.Printf("Wrote version to %s/version.txt\n", binDir)
	return result
}

func main() {
	build()
}
