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
	"github.com/dghubble/trie"
	"github.com/minio/highwayhash"
	"gopkg.in/gookit/color.v1"
)

// Key used as seed for the HighwayHash algorithm.
// This is hardcoded to ensure consistent hashes for files across runs.
const HH_KEY = "E9ECA1531393D174DFEA70CC5BAA4FCE5FC599D08ECB36B9961489985A64D3AE"

func PrintUsage() {
	fmt.Println("Usage: dupes DIRECTORY")
	fmt.Println("DIRECTORY is the directory that will be recursively searched for duplicate files")
}

func PrintDupes(t *trie.RuneTrie) {
	t.Walk(
		func(k string, d interface{}) error {
			if d != nil {
				dupes := d.([]string)
				if len(dupes) > 1 {
					color.Blue.Printf("Hash: %x\n", k)
					for i, f := range dupes {
						color.Red.Printf("\t%d ", i + 1)
						color.Yellow.Printf("%s\n", f)
					}
					fmt.Println()
				}
			}
			return nil
		})
}

func AddEntryToTrie(key string, path string, trie *trie.RuneTrie) {
	dupes := make([]string, 1)
	dupes[0] = path
	trie.Put(key, dupes)
}

func GetSingleReader(path string) (io.Reader, error) {
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

func GetDualReaders(path string) (io.Reader, io.Reader, error) {
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

func ComputeXXHash(r io.Reader) (string, error) {
	h := xxhash.New64()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func ComputeHighwayHash(r io.Reader) (string, error) {
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

func main() {
	args := os.Args[1:]

	if len(args) < 1 {
		PrintUsage()
		os.Exit(1)
	}

	h1Trie := trie.NewRuneTrie()
	h2Trie := trie.NewRuneTrie()
	dupeCount := 0
	fileCount := 0
	prevTime := time.Now().Unix()
	err := filepath.Walk(args[0],
		func(path string, info os.FileInfo, err error) error {
			fileCount++
			currTime := time.Now().Unix()
			if currTime - prevTime >= 5 {
				fmt.Println("Files processed:", fileCount)
				prevTime = currTime
			}

			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			r1, r2, err := GetDualReaders(path)
			if err != nil {
				fmt.Println("Error opening file", path)
				return err
			}

			hash1String, err := ComputeXXHash(r1)
			if err != nil {
				fmt.Println("Error computing xxHash")
				return err
			}

			if exists := h1Trie.Get(hash1String); exists != nil {
				// Compute hash2 of previouly seen file and add it to the trie
				r3, err := GetSingleReader(exists.(string))
				if err != nil {
					fmt.Println("Error opening file", path)
				}

				hash2StringPrevFile, err := ComputeHighwayHash(r3)
				if err != nil {
					fmt.Println("Error computing HighwayHash")
					return err
				}
				AddEntryToTrie(hash1String+hash2StringPrevFile, exists.(string), h2Trie)

				// Now compute hash2 of the current file
				hash2String, err := ComputeHighwayHash(r2)
				if err != nil {
					fmt.Println("Error computing HighwayHash")
					return err
				}

				if d := h2Trie.Get(hash1String + hash2String); d != nil {
					dupeCount++
					dupes := d.([]string)
					dupes = append(dupes, path)
					h2Trie.Put(hash1String+hash2String, dupes)
				} else {
					AddEntryToTrie(hash1String+hash2String, path, h2Trie)
				}
			} else {
				h1Trie.Put(hash1String, path)
			}
			return nil
		})

	if err != nil {
		fmt.Printf("Error reading directory %s. Please ensure the directory exists.\n", args[0])
		os.Exit(3)
	}

	if dupeCount > 0 {
		color.Red.Printf("%d Files with duplicates found:\n", dupeCount)
		PrintDupes(h2Trie)
	} else {
		color.Green.Println("No duplicate files exist in the specified directory.")
	}

}
