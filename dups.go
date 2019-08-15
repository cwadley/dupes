package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/gookit/color.v1"
)

type dupInfo struct {
	hasDupes bool
	dupes    []string
}

func PrintUsage() {
	fmt.Println("Usage: dups PATH")
	fmt.Println("PATH is the directory that will be recursively searched for duplicate files")
}

func PrintDupes(h map[string]*dupInfo) {
	for k, v := range h {
		if v.hasDupes {
			color.Blue.Printf("Hash: %x\n", k)
			for _, f := range v.dupes {
				color.Yellow.Printf("\t%s\n", f)
			}
			fmt.Println()
		}
	}
}

func main() {
	args := os.Args[1:]

	if len(args) != 1 {
		PrintUsage()
		os.Exit(1)
	}

	hashes := map[string]*dupInfo{}
	dupesExist := false
	err := filepath.Walk(args[0],
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			f, err := os.Open(path)
			if err != nil {
				fmt.Println("Error opening file %s", path)
				return err
			}
			defer f.Close()

			h := md5.New()
			if _, err := io.Copy(h, f); err != nil {
				fmt.Println("Error reading file %s", path)
				return err
			}

			hashString := string(h.Sum(nil))
			if val, ok := hashes[hashString]; ok {
				dupesExist = true
				val.hasDupes = true
				val.dupes = append(val.dupes, path)
			} else {
				var d dupInfo
				d.hasDupes = false
				d.dupes = make([]string, 1)
				d.dupes[0] = path
				hashes[hashString] = &d
			}
			return nil
		})

	if err != nil {
		fmt.Println("Error reading directory %s. Please ensure the directory exists.", args[0])
		os.Exit(2)
	}

	if dupesExist {
		fmt.Println("\nDuplicate files:")
		PrintDupes(hashes)
	} else {
		fmt.Println("No duplicate files exist in the specified directory.")
	}

}
