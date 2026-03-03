package kustomizeutil

import (
	"os"
	"path/filepath"
)

var kustomizationFileNames = []string{
	"kustomization.yaml",
	"kustomization.yml",
	"Kustomization",
}

func IsKustomizationFileName(name string) bool {
	for _, k := range kustomizationFileNames {
		if name == k {
			return true
		}
	}

	return false
}

func HasKustomizationFile(dir string) bool {
	for _, k := range kustomizationFileNames {
		if _, err := os.Stat(filepath.Join(dir, k)); err == nil {
			return true
		}
	}

	return false
}
