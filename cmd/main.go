package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	got "github.com/ljpurcell/got/internal"
	cfg "github.com/ljpurcell/got/internal/config"
	"github.com/ljpurcell/got/internal/utils"
)

const READ_WRITE_PERM = 0700

type Command struct {
	Name  string
	Short string
	Long  string
	Help  string
	Run   func([]string)
}

func main() {
	Execute()
}

func Execute() {
	if len(os.Args) < 2 {
		utils.ExitWithError("Not enough arguments")
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

func UnknownCommand(cmd string) *Command {
	return &Command{
		Name:  "unknown",
		Short: "Got did not recognise the subcommand",
		Long:  "Got did not recognise the subcommand; try running \"got --help\" for more",
		Run: func(s []string) {
			utils.ExitWithError("Got did not recognise the subcommand %q", cmd)
		},
	}
}

func InitCommand() *Command {
	return &Command{
		Name:  "init",
		Short: "Initialises a got repository",
		Long:  "Initialises a got repository with a hidden .got file to hold data",
		Run: func(s []string) {
			if utils.Exists(cfg.GOT_REPO) {
				if utils.IsDir(cfg.GOT_REPO) {
					utils.ExitWithError("%q repo already initialised", cfg.GOT_REPO)
				}
				utils.ExitWithError("File named %q already Exists", cfg.GOT_REPO)
			}

			heads := filepath.Join(cfg.GOT_REPO, "refs", "heads")
			os.MkdirAll(heads, READ_WRITE_PERM)

			headFile := filepath.Join(cfg.GOT_REPO, "HEAD")
			head, err := os.Create(headFile)
			if err != nil {
				utils.ExitWithError("Could not create HEAD file: %v", err)
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
				utils.ExitWithError("Not enough arguments to add command")
			}

			indexPath := filepath.Join(cfg.GOT_REPO, "index")
			indexMap := make(map[string]string)

			if !utils.Exists(indexPath) {
				indexFile, err := os.Create(indexPath)
				if err != nil {
					utils.ExitWithError("Could not create index file: %v", err)
				}
				defer indexFile.Close()
			} else {

				indexFile, err := os.Open(indexPath)
				if err != nil {
					utils.ExitWithError("Could not read index file for add command: ", err)
				}
				defer indexFile.Close()

				decoder := json.NewDecoder(indexFile)
				decoder.Decode(&indexMap)
			}

			for _, obj := range s {

				id, _ := got.HashObject(obj)
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
				utils.ExitWithError("You can only pass exactly one argument [commit message] to this command")
			}

			// 1. Generate an up to date tree hash for all listings in index
			indexPath := filepath.Join(cfg.GOT_REPO, "index")

			if !utils.Exists(indexPath) {
				utils.ExitWithError("No index file. Nothing staged to commit")
			}

			index, err := os.Open(indexPath)
			if err != nil {
				utils.ExitWithError("Could not open index for commit command: %v", err)
			}
			indexMap := make(map[string]string)
			decoder := json.NewDecoder(index)
			decoder.Decode(&indexMap)

			if len(indexMap) == 0 {
				utils.ExitWithError("Index file empty. Nothing staged to commit")
			}

			var tree string
			for file := range indexMap {
				id, objectType := got.HashObject(file)
				tree += fmt.Sprintf("%v %v %v %v\n", 100644, id, objectType, file)
			}

			toHash := fmt.Sprintf("tree %d\u0000%v", len(tree), tree)
			hasher := sha1.New()
			hasher.Write([]byte(toHash))
			snapshotId := hex.EncodeToString(hasher.Sum(nil))

			// 2. get parent commit, if Exists
			headRef, err := os.ReadFile(filepath.Join(cfg.GOT_REPO, "HEAD"))
			if err != nil {
				utils.ExitWithError("Could not read content from HEAD file: %v", err)
			}

			ref := strings.Split(string(headRef), ":")
			pathBits := strings.Split(strings.TrimSpace(ref[1]), "/")
			pathToRef := append([]string{cfg.GOT_REPO}, pathBits...)
			path := filepath.Join(pathToRef...)

			parentId := ""
			if utils.Exists(path) {
				contents, err := os.ReadFile(path)
				parentId = string(contents)
				if err != nil {
					utils.ExitWithError("Could not read file at %v: %v")
				}
			}

			// 3. Create commit
			commit := got.CreateCommit(snapshotId, parentId, s[0])

			// Post Commit:
			// 1. Clear index
			if err := os.Truncate(indexPath, 0); err != nil {
				utils.ExitWithError("Could not clear index file: %v", err)
			}

			// 2. Update hash pointed at in HEAD
			if utils.Exists(path) {
				if err := os.Truncate(path, 0); err != nil {
					utils.ExitWithError("Could not clear ref file %v: %v", path, err)
				}
			}

			if err := os.WriteFile(path, []byte(commit.Id), READ_WRITE_PERM); err != nil {
				utils.ExitWithError("Could not write commit id %v to %v file: %v", commitId, path, err)
			}
		},
	}
}
