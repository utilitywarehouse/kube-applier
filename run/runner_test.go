package run

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterOutNamespaces(t *testing.T) {
	dirs := []string{
		"/path/aa",
		"/path/bb",
		"/path/cc",
	}
	ignoredNs := []string{"bb"}

	want := []string{
		"/path/aa",
		"/path/cc",
	}

	res := filterOutNamespaces(ignoredNs, dirs)

	assert.Equal(t, want, res)
}

func TestFilterOutNamespacesWithNilNamespaces(t *testing.T) {
	ns := []string{
		"/path/aa",
		"/path/bb",
		"/path/cc",
	}
	want := []string{
		"/path/aa",
		"/path/bb",
		"/path/cc",
	}

	res := filterOutNamespaces(nil, ns)

	assert.Equal(t, want, res)
}
