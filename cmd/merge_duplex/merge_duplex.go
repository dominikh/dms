// merge_duplex merges pairs of two TIFFs into one multi-page TIFF.
//
// It depends on tiffcp from libtiff to do the actual work.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
)

func die(f string, v ...any) {
	warn(f, v...)
	os.Exit(1)
}

func warn(f string, v ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", v...)
}

func main() {
	if len(os.Args) == 0 {
		return
	}

	files := os.Args[1:]
	if len(files)%2 != 0 {
		die("require even number of pages, got %d", len(files))
	}

	sort.Strings(files)

	if err := os.MkdirAll("merged", 0777); err != nil {
		die("couldn't create output directory: %s", err)
	}

	var failed atomic.Bool
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for i := 0; i < len(files); i += 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			p1, p2 := files[i], files[i+1]

			// Swap back and front because of the way we feed the ADF.
			p1, p2 = p2, p1

			fmt.Printf("merging %s and %s\n", p1, p2)

			out := fmt.Sprintf("merged/%s-%s.tif", path.Base(p1), path.Base(p2))
			if err := exec.Command("tiffcp", "-c", "lzw:1:1", p1, p2, out).Run(); err != nil {
				failed.Store(true)
				warn("could not merge %s and %s: %s", p1, p2, err)
			}
		}(i)
	}
	wg.Wait()
	if failed.Load() {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
