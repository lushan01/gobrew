package gobrew

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/kevincobain2000/gobrew/utils"
)

const (
	goBrewDir     string = ".gobrew"
	registryPath  string = "https://golang.org/dl/"
	fetchTagsRepo string = "https://github.com/golang/go"
)

// Command ...
type Command interface {
	ListVersions()
	ListRemoteVersions()
	CurrentVersion() string
	Uninstall(version string)
	Install(version string)
	Use(version string)
	Helper
}

// GoBrew struct
type GoBrew struct {
	homeDir       string
	installDir    string
	versionsDir   string
	currentDir    string
	currentBinDir string
	currentGoDir  string
	downloadsDir  string
	Command
}

// Helper ...
type Helper interface {
	getArch() string
	existsVersion(version string) bool
	cleanVersionDir(version string)
	mkdirs(version string)
	getVersionDir(version string) string
	downloadAndExtract(version string)
	changeSymblinkGoBin(version string)
	changeSymblinkGo(version string)
}

var gb GoBrew

// NewGoBrew instance
func NewGoBrew() GoBrew {
	gb.homeDir = os.Getenv("HOME")
	gb.installDir = filepath.Join(gb.homeDir, goBrewDir)
	gb.versionsDir = filepath.Join(gb.installDir, "versions")
	gb.currentDir = filepath.Join(gb.installDir, "current")
	gb.currentBinDir = filepath.Join(gb.installDir, "current", "bin")
	gb.currentGoDir = filepath.Join(gb.installDir, "current", "go")
	gb.downloadsDir = filepath.Join(gb.installDir, "downloads")

	return gb
}

func (gb *GoBrew) getArch() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// ListVersions that are installed by dir ls
// highlight the version that is currently symbolic linked
func (gb *GoBrew) ListVersions() {
	files, err := ioutil.ReadDir(gb.versionsDir)
	if err != nil {
		log.Fatalf("[Error]: List versions failed: %s", err)
	}
	cv := gb.CurrentVersion()
	for _, f := range files {
		version := f.Name()
		if version == cv {
			version = cv + "*"
		}
		log.Println(version)
	}

	if cv != "" {
		log.Println()
		log.Printf("current: %s", cv)
	}
}

// ListRemoteVersions that are installed by dir ls
func (gb *GoBrew) ListRemoteVersions() {
	cmd := exec.Command(
		"git",
		"ls-remote",
		"--sort=version:refname",
		"--tags",
		fetchTagsRepo,
		"go*")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("[Error]: List remote versions failed: %s", err)
	}
	tagsRaw := utils.BytesToString(output)
	r, _ := regexp.Compile("tags/go.*")
	matches := r.FindAllString(tagsRaw, -1)
	for _, match := range matches {
		versionTag := strings.ReplaceAll(match, "tags/go", "")
		log.Println(versionTag)
	}
}

func (gb *GoBrew) existsVersion(version string) bool {
	path := filepath.Join(gb.versionsDir, version, "go")
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

// CurrentVersion get current version from symb link
func (gb *GoBrew) CurrentVersion() string {

	fp, err := filepath.EvalSymlinks(gb.currentBinDir)
	if err != nil {
		return ""
	}

	version := strings.ReplaceAll(fp, "/go/bin", "")
	version = strings.ReplaceAll(version, gb.versionsDir, "")
	version = strings.ReplaceAll(version, "/", "")
	return version
}

// Uninstall the given version of go
func (gb *GoBrew) Uninstall(version string) {
	if version == "" {
		log.Fatal("[Error] No version provided")
	}
	if gb.CurrentVersion() == version {
		log.Fatalf("[Error] Version: %s you are trying to remove is your current version. Please use a different version first before uninstalling the current version", version)
		return
	}
	if gb.existsVersion(version) == false {
		log.Fatalf("[Error] Version: %s you are trying to remove is not installed", version)
	}
	gb.cleanVersionDir(version)
	log.Printf("[Success] Version: %s uninstalled", version)
}

func (gb *GoBrew) cleanVersionDir(version string) {
	os.RemoveAll(gb.getVersionDir(version))
}

func (gb *GoBrew) cleanDownloadsDir() {
	os.RemoveAll(gb.downloadsDir)
}

// Install the given version of go
func (gb *GoBrew) Install(version string) {
	if version == "" {
		log.Fatal("[Error] No version provided")
	}
	gb.mkdirs(version)
	if gb.existsVersion(version) == true {
		log.Printf("[Info] Version: %s exists", version)
		return
	}

	log.Printf("[Info] Downloading version: %s", version)
	gb.downloadAndExtract(version)
	gb.cleanDownloadsDir()
	log.Printf("[Success] Downloaded version: %s", version)
}

// Use a version
func (gb *GoBrew) Use(version string) {
	if gb.CurrentVersion() == version {
		log.Printf("[Info] Version: %s is already your current version", version)
		return
	}
	log.Printf("[Info] Changing go version to: %s", version)
	gb.changeSymblinkGoBin(version)
	gb.changeSymblinkGo(version)
	log.Printf("[Success] Changed go version to: %s", version)
}

func (gb *GoBrew) mkdirs(version string) {
	os.MkdirAll(gb.installDir, os.ModePerm)
	os.MkdirAll(gb.currentDir, os.ModePerm)
	os.MkdirAll(gb.versionsDir, os.ModePerm)
	os.MkdirAll(gb.getVersionDir(version), os.ModePerm)
	os.MkdirAll(gb.downloadsDir, os.ModePerm)
}

func (gb *GoBrew) getVersionDir(version string) string {
	return filepath.Join(gb.versionsDir, version)
}
func (gb *GoBrew) downloadAndExtract(version string) {
	tarName := "go" + version + "." + gb.getArch() + ".tar.gz"

	downloadURL := registryPath + tarName

	err := utils.Download(
		downloadURL,
		filepath.Join(gb.downloadsDir, tarName))

	if err != nil {
		gb.cleanVersionDir(version)
		log.Printf("[Info]: Downloading version failed: %s", err)
		log.Fatalf("[Error]: Please check connectivity to url: %s", downloadURL)
	}

	cmd := exec.Command(
		"tar",
		"-xf",
		filepath.Join(gb.downloadsDir, tarName),
		"-C",
		gb.getVersionDir(version))

	log.Printf("[Success] Untar to %s", gb.getVersionDir(version))
	_, err = cmd.Output()
	if err != nil {
		// clean up dir
		gb.cleanVersionDir(version)
		log.Printf("[Info]: Untar failed: %s", err)
		log.Fatalf("[Error]: Please check if version exists from url: %s", downloadURL)
	}
}

func (gb *GoBrew) changeSymblinkGoBin(version string) {

	goBinDst := filepath.Join(gb.versionsDir, version, "/go/bin")
	os.RemoveAll(gb.currentBinDir)

	cmd := exec.Command("ln", "-snf", goBinDst, gb.currentBinDir)

	_, err := cmd.Output()
	if err != nil {
		log.Fatalf("[Error]: symbolic link failed: %s", err)
	}

}
func (gb *GoBrew) changeSymblinkGo(version string) {

	os.RemoveAll(gb.currentGoDir)
	versionGoDir := filepath.Join(gb.versionsDir, gb.CurrentVersion(), "go")
	cmd := exec.Command("ln", "-snf", versionGoDir, gb.currentGoDir)

	_, err := cmd.Output()
	if err != nil {
		log.Fatalf("[Error]: symbolic link failed: %s", err)
	}
}
