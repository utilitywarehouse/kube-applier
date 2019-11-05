package sysutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/utilitywarehouse/kube-applier/log"
)

// ListDirs walks the directory tree rooted at the path and adds all non-directory file paths to a []string.
func ListDirs(rootPath string) ([]string, error) {
	var dirs []string
	files, err := ioutil.ReadDir(rootPath)
	if err != nil {
		return dirs, fmt.Errorf("Could not read %s error=(%v)", rootPath, err)
	}

	for _, file := range files {
		if file.IsDir() {
			dirs = append(dirs, filepath.Join(rootPath, file.Name()))
		}
	}
	return dirs, nil
}

// WaitForDir returns when the specified directory is located in the filesystem, or if there is an error opening the directory once it is found.
func WaitForDir(path string, clock ClockInterface, interval time.Duration) error {
	for {
		f, err := os.Stat(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("Error opening the directory at %v: %v", path, err)
			}
		} else if !f.IsDir() {
			return fmt.Errorf("Error: %v is not a directory", path)
		} else {
			break
		}
		clock.Sleep(interval)
	}
	return nil
}

// PruneDirs takes a list of directory paths and omits those that don't match at least one item in a list of patterns
func PruneDirs(dirs []string, filters []string) []string {
	if len(filters) == 0 {
		return dirs
	}

	var prunedDirs []string
	for _, dir := range dirs {
		for _, filter := range filters {
			matched, err := filepath.Match(path.Join(filepath.Dir(dir), filter), dir)
			if err != nil {
				log.Logger.Error(err.Error())
			} else if matched {
				prunedDirs = append(prunedDirs, dir)
			}
		}
	}

	return prunedDirs
}
