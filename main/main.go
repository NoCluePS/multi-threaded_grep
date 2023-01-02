package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/NoCluePS/mgrep/worker"
	"github.com/NoCluePS/mgrep/worklist"
	"github.com/alexflint/go-arg"
)

func findDir(wl *worklist.Worklist, path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Println("Readdir error:", err)
		return
	}

	for _, entry := range entries {
		nextPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			findDir(wl, nextPath)
		} else {
			wl.Add(worklist.NewJob(nextPath))
		}
	}
}

var args struct {
	SearchTerm string `arg:"positional,required"`
	SearchDir string `arg:"positional"`
}

func main() {
	arg.MustParse(&args)

	var workerWg sync.WaitGroup

	wl := worklist.New(100)

	results := make(chan worker.Result, 100)
	numWorkers := 10

	workerWg.Add(1)
	go func ()  {
		defer workerWg.Done()
		findDir(&wl, args.SearchDir)	
		wl.Finalize(numWorkers)
	}()

	for i := 0; i < numWorkers; i++ {
		workerWg.Add(1)

		go func() {
			defer workerWg.Done()
			for {
				workEntry := wl.Next()
				if workEntry.Path != "" {
					workerResult := worker.FindInFile(workEntry.Path, args.SearchTerm)
					if workerResult != nil {
						for _, r := range workerResult.Inner {
							results <- r
						}
					}
				} else {
					return
				}
			}
		}()
	}
	
	blockWorkersWg := make(chan struct{})
	go func() {
		workerWg.Wait()
		close(blockWorkersWg)
	}()

	var displayWg sync.WaitGroup

	displayWg.Add(1)
	go func() {
		for {
			select {
			case result := <-results:
				fmt.Printf("%v[%v]:%v\n", result.Path, result.LineNum, strings.Trim(result.Line, " "))
			case <-blockWorkersWg:
				if len(results) == 0 {
					displayWg.Done()
					return
				}
			}
		}
	}()
	displayWg.Wait()

}