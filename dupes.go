package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/OneOfOne/xxhash"
	"github.com/minio/highwayhash"
	"github.com/cornelk/hashmap"
	"gopkg.in/gookit/color.v1"
)

// Key used as seed for the HighwayHash algorithm.
// This is hardcoded to ensure consistent hashes for files across runs.
const HH_KEY = "E9ECA1531393D174DFEA70CC5BAA4FCE5FC599D08ECB36B9961489985A64D3AE"

type dupe struct {
	Hash  string   `json:"hash"`
	Files []string `json:"files"`
}

func printUsage() {
	fmt.Println("Usage: dupes [OPTIONS] <dupe_directory>")
	fmt.Println("\tdupe_directory is the directory that will be recursively searched for duplicate files")
	fmt.Println("Options:")
	fmt.Println("\t-j, --json <path> (Optional)")
	fmt.Println("\t\tOutputs results as JSON to the specified file path")
}

func printDupes(hm *hashmap.HashMap, json_output bool, json_file string) error {
	var json_dupes []dupe
	for e := range hm.Iter() {
		if e.Value != nil {
			dupes := e.Value.([]string)
			if len(dupes) > 1 {
				color.Blue.Printf("Hash: %x\n", e.Key.(string))
				for i, f := range dupes {
					color.Red.Printf("\t%d ", i+1)
					color.Yellow.Printf("%s\n", f)
				}
				fmt.Println()

				if json_output {
					var curr_dupe dupe
					curr_dupe.Hash = e.Key.(string)
					curr_dupe.Files = dupes
					json_dupes = append(json_dupes, curr_dupe)
				}
			}
		}
	}

	if json_output {
		json_data, err := json.Marshal(json_dupes)
		if err != nil {
			fmt.Println("Error marshalling output JSON")
			return err
		}
		err = ioutil.WriteFile(json_file, json_data, 0644)
		if err != nil {
			fmt.Println("Error writing JSON file, please check permissions and that the directory exists.")
			return err
		}
	}
	return nil
}

func addDupesToHashMap(key string, path string, hm *hashmap.HashMap) {
	dupes := make([]string, 1)
	dupes[0] = path
	hm.Set(key, dupes)
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

func processFile(path string, info os.FileInfo, err error, h1HM *hashmap.HashMap, h2HM *hashmap.HashMap,
	dupeCount *int64, fileCount *int64, prevTime *int64) error {
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
		return nil
	}

	hash1String, err := computeXXHash(r1)
	if err != nil {
		fmt.Println("Error computing xxHash")
		return err
	}

	if val, ok := h1HM.GetStringKey(hash1String); ok {
		// Compute hash2 of previouly seen file and add it to the trie
		r3, err := getSingleReader(val.(string))
		if err != nil {
			fmt.Println("Error opening file", path)
			return nil
		}

		hash2StringPrevFile, err := computeHighwayHash(r3)
		if err != nil {
			fmt.Println("Error computing HighwayHash")
			return err
		}
		addDupesToHashMap(hash1String + hash2StringPrevFile, val.(string), h2HM)

		// Now compute hash2 of the current file
		hash2String, err := computeHighwayHash(r2)
		if err != nil {
			fmt.Println("Error computing HighwayHash")
			return err
		}

		if d, ok := h2HM.GetStringKey(hash1String + hash2String); ok {
			*dupeCount++
			dupes := d.([]string)
			dupes = append(dupes, path)
			h2HM.Set(hash1String + hash2String, dupes)
		} else {
			addDupesToHashMap(hash1String + hash2String, path, h2HM)
		}
	} else {
		h1HM.Set(hash1String, path)
	}
	return nil
}

func main() {
	args := os.Args[1:]

	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	json_output := false
	var json_file string
	dupeDir := ""
	for i := 0; i < len(args); i++ {
		if string(args[i][0]) == "-" {
			switch flag := string(args[i][1:]); flag {
			case "j", "-json":
				if i+1 >= len(args) {
					fmt.Println("Error: No JSON output file specified")
					printUsage()
					os.Exit(1)
				}
				json_output = true
				json_file = args[i+1]
				i++
			default:
				fmt.Println("Error: Invalid flag", args[i])
				printUsage()
				os.Exit(1)
			}
		} else {
			dupeDir = args[i]
		}
	}

	if dupeDir == "" {
		fmt.Println("Error: No directory specified to scan for duplicate files")
		printUsage()
		os.Exit(1)
	}

	h1HM := hashmap.New(1024)
	h2HM := hashmap.New(1024)
	var dupeCount int64
	var fileCount int64
	prevTime := time.Now().Unix()
	err := filepath.Walk(dupeDir,
		func(path string, info os.FileInfo, err error) error {
			return processFile(path, info, err, h1HM, h2HM, &dupeCount, &fileCount, &prevTime)
		})

	if err != nil {
		os.Exit(3)
	}

	if dupeCount > 0 {
		color.Red.Printf("%d Files with duplicates found:\n", dupeCount)
		_ = printDupes(h2HM, json_output, json_file)
	} else {
		color.Green.Println("No duplicate files exist in the specified directory.")
	}

}
