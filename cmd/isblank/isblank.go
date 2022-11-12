package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"golang.org/x/image/tiff"
)

func die(f string, v ...any) {
	warn(f, v...)
	os.Exit(1)
}

func warn(f string, v ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", v...)
}

func dimensions(path string) (width, height int, err error) {
	f, err := os.Open(os.Args[1])
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	c, err := tiff.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return c.Width, c.Height, nil
}

func edges(path string) (int, error) {
	b, err := exec.Command("convert", path, "-edge", "2", "(", "+clone", "-evaluate", "set", "0", ")", "-metric", "AE", "-compare", "-format", "%[distortion]", "info:").Output()
	if err != nil {
		return 0, err
	}
	f, err := strconv.ParseFloat(string(b), 64)
	return int(f), err
}

func main() {
	w, h, err := dimensions(os.Args[1])
	if err != nil {
		die("couldn't get document's dimensions: %s", err)
	}
	e, err := edges(os.Args[1])
	if err != nil {
		die("couldn't compute document's edginess: %s", err)
	}
	r := float64(e) / float64(w*h)

	fmt.Println(r)

	if r > 0.03 {
		// The least edgy non-empty page we saw had r == 0.031214395294534195. Unfortunately this is lower than the edgiest
		// blank page we saw, which had r == 0.04107169782337893.
		os.Exit(0)
	} else {
		os.Exit(1)
	}
}
