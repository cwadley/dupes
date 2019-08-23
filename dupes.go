package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/OneOfOne/xxhash"
	"github.com/minio/highwayhash"
	"github.com/xiaonanln/go-trie-tst"
	"gopkg.in/gookit/color.v1"
)

// Key used as seed for the HighwayHash algorithm.
// This is hardcoded to ensure consistent hashes for files across runs.
const HH_KEY = "E9ECA1531393D174DFEA70CC5BAA4FCE5FC599D08ECB36B9961489985A64D3AE"

func printUsage() {
	fmt.Println("Usage: dupes DIRECTORY")
	fmt.Println("DIRECTORY is the directory that will be recursively searched for duplicate files")
}

func printDupes(t *trietst.TST) {
	t.ForEach(
		func(k string, d interface{}) {
			if d != nil {
				dupes := d.([]string)
				if len(dupes) > 1 {
					color.Blue.Printf("Hash: %x\n", k)
					for i, f := range dupes {
						color.Red.Printf("\t%d ", i+1)
						color.Yellow.Printf("%s\n", f)
					}
					fmt.Println()
				}
			}
		})
}

func addDupesToTST(key string, path string, t *trietst.TST) {
	dupes := make([]string, 1)
	dupes[0] = path
	t.Set(key, dupes)
}

func getSingleReader(path string) (io.Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(b)
	return r, nil
}

func getDualReaders(path string) (io.Reader, io.Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, nil, err
	}
	r1 := bytes.NewReader(b)
	r2 := bytes.NewReader(b)
	return r1, r2, nil
}

func computeXXHash(r io.Reader) (string, error) {
	h := xxhash.New64()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func computeHighwayHash(r io.Reader) (string, error) {
	hhKey, err := hex.DecodeString(HH_KEY)
	if err != nil {
		return "", err
	}

	h, err := highwayhash.New(hhKey)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func processFile(path string, info os.FileInfo, err error, h1TST *trietst.TST, h2TST *trietst.TST,
	dupeCount *int64, fileCount *int64, prevTime *int64) error {
	fmt.Println("fileCount", *fileCount)
	*fileCount++
	currTime := time.Now().Unix()
	if currTime - *prevTime >= 5 {
		fmt.Println("Files processed:", *fileCount)
		*prevTime = currTime
	}

	if err != nil {
		return err
	}

	if info.IsDir() {
		return nil
	}

	r1, r2, err := getDualReaders(path)
	if err != nil {
		fmt.Println("Error opening file", path)
		return err
	}

	hash1String, err := computeXXHash(r1)
	if err != nil {
		fmt.Println("Error computing xxHash")
		return err
	}

	if exists := h1TST.Get(hash1String); exists != nil {
		// Compute hash2 of previouly seen file and add it to the trie
		r3, err := getSingleReader(exists.(string))
		if err != nil {
			fmt.Println("Error opening file", path)
		}

		hash2StringPrevFile, err := computeHighwayHash(r3)
		if err != nil {
			fmt.Println("Error computing HighwayHash")
			return err
		}
		addDupesToTST(hash1String + hash2StringPrevFile, exists.(string), h2TST)

		// Now compute hash2 of the current file
		hash2String, err := computeHighwayHash(r2)
		if err != nil {
			fmt.Println("Error computing HighwayHash")
			return err
		}

		if d := h2TST.Get(hash1String + hash2String); d != nil {
			*dupeCount++
			dupes := d.([]string)
			dupes = append(dupes, path)
			h2TST.Set(hash1String + hash2String, dupes)
		} else {
			addDupesToTST(hash1String + hash2String, path, h2TST)
		}
	} else {
		h1TST.Set(hash1String, path)
	}
	fmt.Println("dupeCount", *dupeCount)
	return nil
}

func main() {
	args := os.Args[1:]

	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	var h1TST trietst.TST
	var h2TST trietst.TST
	var dupeCount int64
	var fileCount int64
	prevTime := time.Now().Unix()
	err := filepath.Walk(args[0],
		func(path string, info os.FileInfo, err error) error {
			return processFile(path, info, err, &h1TST, &h2TST, &dupeCount, &fileCount, &prevTime)
		})

	if err != nil {
		fmt.Printf("Error reading directory %s. Please ensure the directory exists.\n", args[0])
		os.Exit(3)
	}

	if dupeCount > 0 {
		color.Red.Printf("%d Files with duplicates found:\n", dupeCount)
		printDupes(&h2TST)
	} else {
		color.Green.Println("No duplicate files exist in the specified directory.")
	}

}
