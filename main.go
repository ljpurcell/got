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
	"strings"
	"time"
)

const GOT_REPO = ".got"
const READ_WRITE_PERM = 0700

func main() {
	Execute()
}

type Command struct {
	Name  string
	Short string
	Long  string
	Help  string
	Run   func([]string)
}

type GotObject struct {
	id string
}

type Commit struct {
	object    GotObject
	author    string
	createdAt time.Time
	message   string
	parentId  string
	treeId    string
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

			heads := filepath.Join(GOT_REPO, "refs", "heads")
			os.MkdirAll(heads, READ_WRITE_PERM)

			headFile := filepath.Join(GOT_REPO, "HEAD")
			head, err := os.Create(headFile)
			if err != nil {
				exitWithError("Could not create HEAD file: %v", err)
			}
			defer head.Close()
			os.WriteFile(headFile, []byte("ref: refs/heads/main"), READ_WRITE_PERM)

			fmt.Fprintf(os.Stdout, "Initialised an empty got repository\n")
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

				var id string
				if isDir(obj) {
					id = hashTree(obj, true)
				} else {
					id = hashBlob(obj, true)
				}

				indexMap[obj] = id
			}

			var b bytes.Buffer
			encoder := json.NewEncoder(&b)
			encoder.Encode(indexMap)

			os.WriteFile(indexPath, b.Bytes(), READ_WRITE_PERM)
		},
	}
}

func CommitCommand() *Command {
	return &Command{
		Name:  "commit",
		Short: "Commit the current index",
		Long:  "Create a commit (snapshot) of the current state of the objects listed in the index",
		Run: func(s []string) {
			if len(s) != 1 {
				exitWithError("You can only pass exactly one argument [commit message] to this command")
			}

			// 1. Generate an up to date tree hash for all listings in index
			indexPath := filepath.Join(GOT_REPO, "index")

			if !exists(indexPath) {
				exitWithError("No index file. Nothing staged to commit")
			}

			index, err := os.Open(indexPath)
			if err != nil {
				exitWithError("Could not open index for commit command: %v", err)
			}
			indexMap := make(map[string]string)
			decoder := json.NewDecoder(index)
			decoder.Decode(&indexMap)

			if len(indexMap) == 0 {
				exitWithError("Index file empty. Nothing staged to commit")
			}

			var tree string
			for file := range indexMap {
				if isDir(file) {
					treeId := hashTree(file, true)
					tree += fmt.Sprintf("%v tree %v %v\n", 100644, treeId, file)
				} else {
					blobId := hashBlob(file, true)
					tree += fmt.Sprintf("%v blob %v %v\n", 100644, blobId, file)
				}
			}

			toHash := fmt.Sprintf("tree %d\u0000%v", len(tree), tree)
			hasher := sha1.New()
			hasher.Write([]byte(toHash))
			snapshotId := hex.EncodeToString(hasher.Sum(nil))

			// 2. get parent commit, if exists
			headRef, err := os.ReadFile(filepath.Join(GOT_REPO, "HEAD"))
			if err != nil {
				exitWithError("Could not read content from HEAD file: %v", err)
			}

			ref := strings.Split(string(headRef), ":")
			pathBits := strings.Split(strings.TrimSpace(ref[1]), "/")
			pathToRef := append([]string{GOT_REPO}, pathBits...)
			path := filepath.Join(pathToRef...)

			parentId := ""
			if exists(path) {
				contents, err := os.ReadFile(path)
				parentId = string(contents)
				if err != nil {
					exitWithError("Could not read file at %v: %v")
				}
			}

			// 3. Create commit
			c := createCommit(snapshotId, parentId, s[0])

			// Post Commit:
			// 1. Clear index
			if err := os.Truncate(indexPath, 0); err != nil {
				exitWithError("Could not clear index file: %v", err)
			}

			// 2. Update hash pointed at in HEAD
			if exists(path) {
				if err := os.Truncate(path, 0); err != nil {
					exitWithError("Could not clear ref file %v: %v", path, err)
				}
			}

			commitId := c.object.id

			if err := os.WriteFile(path, []byte(commitId), READ_WRITE_PERM); err != nil {
				exitWithError("Could not write commit id %v to %v file: %v", commitId, path, err)
			}
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
	case "commit":
		cmd = CommitCommand()
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

	if write {
		objDir := filepath.Join(GOT_REPO, "objects", id[:2])
		objFile := filepath.Join(objDir, id[2:])

		os.MkdirAll(objDir, READ_WRITE_PERM)
		file, err := os.Create(objFile)
		if err != nil {
			exitWithError("Could not write object %q (tree) using name %q in directory %q", dir, objFile, objDir)
		}

		defer file.Close()

		var b bytes.Buffer
		compressor := zlib.NewWriter(&b)
		compressor.Write([]byte(toHash))
		compressor.Close()

		err = os.WriteFile(objFile, b.Bytes(), READ_WRITE_PERM)
		if err != nil {
			exitWithError("Could not write compressed contents of %v to %v", dir, objFile)
		}
	}

	return id
}

func createCommit(tree string, parentId string, msg string) *Commit {
	parentListing := ""
	if parentId != "" {
		parentListing = fmt.Sprintf("parent %v", parentId)
	}

	committer := "ljpurcell" // TODO work on Got config

	data := fmt.Sprintf("tree %v\n%v\ncommiter %v\n\n%v", tree, parentListing, committer, msg)

	toHash := fmt.Sprintf("commit %d\u0000%v", len(data), data)
	hasher := sha1.New()
	hasher.Write([]byte(toHash))
	id := hex.EncodeToString(hasher.Sum(nil))

	objDir := filepath.Join(GOT_REPO, "objects", id[:2])
	objFile := filepath.Join(objDir, id[2:])

	os.MkdirAll(objDir, READ_WRITE_PERM)
	file, err := os.Create(objFile)
	if err != nil {
		exitWithError("Could not write object (commit) using name %q in directory %q", objFile, objDir)
	}

	defer file.Close()

	var b bytes.Buffer
	compressor := zlib.NewWriter(&b)
	compressor.Write([]byte(toHash))
	compressor.Close()

	err = os.WriteFile(objFile, b.Bytes(), READ_WRITE_PERM)
	if err != nil {
		exitWithError("Could not write compressed contents of commit with message %q to %v", msg, objFile)
	}

	return &Commit{
		object: GotObject{
			id: id,
		},
		author:    "ljpurcell",
		createdAt: time.Now(),
		message:   msg,
		parentId:  "",
		treeId:    tree,
	}
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
