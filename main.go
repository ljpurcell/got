package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const GOT_REPO = ".got"
const READ_WRITE_PERM = 0700

func main() {
	hashTree("test", true)
}

type Command struct {
	Name  string
	Short string
	Long  string
	Help  string
	Run   func([]string)
}

func UnknownCommand(cmd string) *Command {
	return &Command{
		Name:  "unknown",
		Short: "Got did not recognise the subcommand",
		Long:  "Got did not recognise the subcommand; try running \"got --help\" for more",
		Run: func(s []string) {
			exitWithError("Got did not recognise the subcommand %q", cmd)
		},
	}
}

func InitCommand() *Command {
	return &Command{
		Name:  "init",
		Short: "Initialises a got repository",
		Long:  "Initialises a got repository with a hidden .got file to hold data",
		Run: func(s []string) {
			if exists(GOT_REPO) {
				if isDir(GOT_REPO) {
					exitWithError("%q repo already initialised", GOT_REPO)
				}
				exitWithError("File named %q already exists", GOT_REPO)
			}

			os.Mkdir(GOT_REPO, READ_WRITE_PERM)
			// TODO: Initialie with commit
		},
	}

}

func AddCommand() *Command {
	return &Command{
		Name:  "add",
		Short: "Add objects to the index",
		Long:  "Add files or directories to the index (staging area)",
		Run: func(s []string) {
			if len(s) < 1 {
				exitWithError("Not enough arguments to add command")
			}

			indexPath := filepath.Join(GOT_REPO, "index")
			indexMap := make(map[string]string)

			if !exists(indexPath) {
				indexFile, err := os.Create(indexPath)
				if err != nil {
					exitWithError("Could not create index file: %v", err)
				}
				defer indexFile.Close()
			} else {

				indexFile, err := os.Open(indexPath)
				if err != nil {
					exitWithError("Could not read index file for add command: ", err)
				}
				defer indexFile.Close()

				decoder := json.NewDecoder(indexFile)
				decoder.Decode(&indexMap)
			}

			for _, obj := range s {
				id := hashBlob(obj, true)
				indexMap[obj] = id
			}

			var b bytes.Buffer
			encoder := json.NewEncoder(&b)
			encoder.Encode(indexMap)

			os.WriteFile(indexPath, b.Bytes(), READ_WRITE_PERM)
		},
	}
}

func Execute() {
	if len(os.Args) < 2 {
		exitWithError("Not enough arguments")
	}

	var cmd *Command
	subCmd := os.Args[1]

	switch subCmd {
	case "init":
		cmd = InitCommand()
	case "add":
		cmd = AddCommand()
	default:
		cmd = UnknownCommand(subCmd)
	}

	cmd.Run(os.Args[2:])
}

// got library
func hashBlob(obj string, write bool) string {
	if !exists(obj) {
		exitWithError("Cannot hash %q. Object doesn't exist", obj)
	}

	if isDir(obj) {
		exitWithError("Cannot call hash blob on %q. Object is a directory", obj)
	}

	info, err := os.Stat(obj)
	if err != nil {
		exitWithError("Could not get file size for hash of %q", obj)
	}

	toHash := fmt.Sprintf("blob %d\u0000", info.Size())
	hasher := sha1.New()
	hasher.Write([]byte(toHash))
	id := hex.EncodeToString(hasher.Sum(nil))

	if write {
		objDir := filepath.Join(GOT_REPO, "objects", id[:2])
		objFile := filepath.Join(objDir, id[2:])

		os.MkdirAll(objDir, READ_WRITE_PERM)
		file, err := os.Create(objFile)
		if err != nil {
			exitWithError("Could not write object %q using name %q in directory %q", obj, objFile, objDir)
		}

		defer file.Close()

		fileContents, err := os.ReadFile(obj)
		if err != nil {
			exitWithError("Could not read contents from file %v for compression", obj)
		}

		var b bytes.Buffer
		compressor := zlib.NewWriter(&b)
		compressor.Write([]byte(fileContents))
		compressor.Close()

		err = os.WriteFile(objFile, b.Bytes(), READ_WRITE_PERM)
		if err != nil {
			exitWithError("Could not write compressed contents of %v to %v", obj, objFile)
		}
	}

	return id
}

func hashTree(dir string, write bool) string {
	if !exists(dir) {
		exitWithError("Cannot hash %q. Object doesn't exist", dir)
	}

	if !isDir(dir) {
		exitWithError("Cannot call hash tree on %q. Object is not a directory", dir)
	}

	var tree string
	files, err := os.ReadDir(dir)
	if err != nil {
		exitWithError("Could not read files for %v: %v", dir, err)
	}

	for _, file := range files {
		filePath := filepath.Join(dir, file.Name())
		if file.IsDir() {
			treeId := hashTree(filePath, true)
			tree += fmt.Sprintf("%v tree %v %v\n", 100644, treeId, file.Name())
		} else {
			blobId := hashBlob(filePath, true)
			tree += fmt.Sprintf("%v blob %v %v\n", 100644, blobId, file.Name())
		}

	}

	toHash := fmt.Sprintf("tree %d\u0000%v", len(tree), tree)
	hasher := sha1.New()
	hasher.Write([]byte(toHash))
	id := hex.EncodeToString(hasher.Sum(nil))
	return id
}

// general utils
func exists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}

		exitWithError("Could not check if file %v exists: %v", path, err)
	}

	return true
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		exitWithError("Could not check if file %v is directory: %v", path, err)
	}

	return info.IsDir()
}

func exitWithError(msg string, args ...interface{}) {
	var output string
	if len(args) > 0 {
		output = fmt.Sprintf(msg+"\n", args...)
	} else {
		output = fmt.Sprintf(msg + "\n")
	}

	fmt.Fprintf(os.Stderr, output)
	os.Exit(1)
}
