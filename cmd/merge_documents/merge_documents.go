package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func die(f string, v ...any) {
	warn(f, v...)
	os.Exit(1)
}

func warn(f string, v ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", v...)
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		die("couldn't find home directory: %s", err)
	}
	f, err := os.Open(filepath.Join(home, ".marked_documents"))
	if err != nil {
		die("couldn't open log: %s", err)
	}
	r := bufio.NewReader(f)

	bases := map[string]struct{}{}
	docs := map[string]struct{}{}

	merges := map[string][]string{}
	dels := map[string]struct{}{}
	deletePages := map[string][]int{}
	var lastBase string
	for {
		l, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			} else {
				die("couldn't read line: %s", err)
			}
		}

		l = l[:len(l)-1]

		parts := strings.Split(l, "\t")
		switch parts[0] {
		case "":
			continue
		case "base":
			merges[parts[1]] = []string{}
			lastBase = parts[1]
			bases[parts[1]] = struct{}{}
		case "merge":
			if lastBase == "" {
				die("saw merge command with no preceding base command")
			}
			merges[lastBase] = append(merges[lastBase], parts[1])
			docs[parts[1]] = struct{}{}
		case "delpage":
			page, err := strconv.Atoi(parts[2])
			if err != nil {
				die("could not parse page number: %s", err)
			}
			deletePages[parts[1]] = append(deletePages[parts[1]], page)
		case "delete":
			dels[parts[1]] = struct{}{}
		default:
			die("saw unknown command %q", parts[0])
		}
	}

	for b := range bases {
		if _, ok := docs[b]; ok {
			die("malformed log: %s occurs both as base and as merge", b)
		}

		if _, ok := dels[b]; ok {
			die("malformed log: %s occurs both as base and as deleted document", b)
		}
	}

	for doc, pages := range deletePages {
		del := ""
		for _, p := range pages {
			// ~p subtracts a page from a range of pages in pdftk. The default range is all pages, so we are subtracting
			// page p, i.e. deleting it. The order of pages doesn't matter, pdftk keeps track of shifting indices.
			// Duplicates are also fine.
			//
			// FIXME(dh): deleting all pages leads to a crash in pdftk
			del += fmt.Sprintf("~%d", p)

			fmt.Printf("deleting page %d in document %s\n", p, doc)
		}
		tmp, err := os.CreateTemp(filepath.Dir(doc), "pdftk*.pdf")
		if err != nil {
			die("couldn't create temporary file: %s", err)
		}
		tmpName := tmp.Name()
		tmp.Close()
		if out, err := exec.Command("pdftk", doc, "cat", del, "output", tmpName).CombinedOutput(); err != nil {
			die("couldn't run pdftk: %s\n%s", err, out)
		}
		if err := os.Rename(tmpName, doc); err != nil {
			die("couldn't rename file: %s", err)
		}
	}

	for doc := range dels {
		fmt.Printf("deleting document %s\n", doc)
		if err := os.Remove(doc); err != nil {
			warn("couldn't delete file %s: %s", doc, err)
		}
	}

	for b, mm := range merges {
		if len(mm) == 0 {
			continue
		}
		fmt.Printf("merging %d documents into %s\n", len(mm), b)

		tmp, err := os.CreateTemp(filepath.Dir(b), "pdfunite*.pdf")
		if err != nil {
			die("couldn't create temporary file: %s", err)
		}
		tmpName := tmp.Name()
		tmp.Close()

		var args []string
		args = append(args, b)
		args = append(args, mm...)
		args = append(args, tmpName)
		// TODO(dh): don't use pdfunite if we already use pdftk, anyway
		if out, err := exec.Command("pdfunite", args...).CombinedOutput(); err != nil {
			die("couldn't run pdfunite: %s\n%s", err, out)
		}
		if err := os.Rename(tmpName, b); err != nil {
			die("couldn't rename file: %s", err)
		}

		for _, m := range mm {
			if err := os.Remove(m); err != nil {
				warn("couldn't delete merged document %s: %s", m, err)
			}
		}
	}
}
