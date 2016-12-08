package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jessevdk/go-flags"
)

const (
	_   = iota
	KiB = 1 << (iota * 10)
	MiB
	GiB
)

var done = make(chan struct{})
var sema = make(chan struct{}, 20)

func cancelled() bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

func main() {
	var options Options
	parser := flags.NewParser(&options, flags.Default)
	parser.Usage = "[OPTIONS] [dir1 dir2 ...]"
	roots, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}
	if len(roots) == 0 {
		roots = []string{"."}
	}

	fmt.Println("Press return, to cancel at any time...")
	go func() {
		os.Stdin.Read(make([]byte, 1))
		close(done)
	}()

	fileSizes := make(chan int64)

	var n sync.WaitGroup
	for _, root := range roots {
		n.Add(1)
		go walkDir(root, &n, fileSizes)
	}
	go func() {
		n.Wait()
		close(fileSizes)
	}()

	var tick <-chan time.Time
	if options.Verbose {
		tick = time.Tick(500 * time.Millisecond)
	}

	var nfiles, nbytes int64
loop:
	for {
		select {
		case <-done:
			for range fileSizes {
				// Do nothing
			}
			return
		case size, ok := <-fileSizes:
			if !ok {
				break loop
			}
			nfiles++
			nbytes += size
		case <-tick:
			printDiskUsage(nfiles, nbytes, &options)
		}
	}

	printDiskUsage(nfiles, nbytes, &options)
}

func printDiskUsage(nfiles, nbytes int64, options *Options) {
	var display string
	var size float64
	switch {
	case options.K:
		display = "KiB"
		size = float64(nbytes) / KiB
	case options.G:
		display = "GiB"
		size = float64(nbytes) / GiB
	default:
		display = "MiB"
		size = float64(nbytes) / MiB
	}
	fmt.Printf("%d files %.2f %s\n", nfiles, size, display)
}

func walkDir(dir string, n *sync.WaitGroup, fileSizes chan<- int64) {
	defer n.Done()
	if cancelled() {
		return
	}
	for _, entry := range dirents(dir) {
		if entry.IsDir() {
			n.Add(1)
			subdir := filepath.Join(dir, entry.Name())
			go walkDir(subdir, n, fileSizes)
		} else {
			fileSizes <- entry.Size()
		}
	}
}

func dirents(dir string) []os.FileInfo {
	select {
	case sema <- struct{}{}:
	case <-done:
		return nil
	}
	defer func() { <-sema }()

	f, err := os.Open(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open: %s\n", err)
		return nil
	}
	defer f.Close()

	entries, err := f.Readdir(0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot readdir: %s\n", err)
		// Don't return: Readdir may return partial results.
	}
	return entries
}
