package sysutil_test

import (
	"fmt"

	"github.com/utilitywarehouse/kube-applier/sysutil"
)

func ExampleListDirs() {
	dirs, err := sysutil.ListDirs("./testdata")
	if err != nil {
		panic(err)
	}

	fmt.Println(dirs)
	// Output: [testdata/parentdir/childdir1/childchilddir testdata/parentdir/childdir1 testdata/parentdir/childdir2 testdata/parentdir]
}
