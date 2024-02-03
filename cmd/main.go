package main

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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
	case "checkout":
		cmd = CheckoutCommand()
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

			index := got.GetIndex()

			for _, file := range s {

				index.UpdateOrAddEntry(file)
			}

			index.Save()
		},
	}
}

func RemoveCommand() *Command {
	return &Command{
		Name:  "remove",
		Short: "Remove objects from the working directory",
		Long:  "Remove files or directories from the index (staging area)",
		Run: func(s []string) {
			if len(s) < 1 {
				utils.ExitWithError("Not enough arguments to add command")
			}

			index := got.GetIndex()

			for _, file := range s {

				index.RemoveFile(file)
			}

			index.Save()
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
			index := got.GetIndex()

			if index.Length() == 0 {
				utils.ExitWithError("Index file empty. Nothing staged to commit")
			}

            entryNames := make([]string, index.Length())

            for i, entry := range index.Entries() {
                entryNames[i] = entry.Name
            }

            fmt.Printf("%v", entryNames)

            slices.Sort(entryNames)

            fmt.Printf("%v", entryNames)

			var tree string
			for _, name := range entryNames {
				id, objectType := got.WriteObject(name)
				tree += fmt.Sprintf("%v %v %v %v\n", 100644, objectType, id, name)
			}

			id, treeString := got.FormatHexId(tree, got.TREE)

			objDir := filepath.Join(cfg.GOT_REPO, "objects", id[:2])
			objFile := filepath.Join(objDir, id[2:])

			os.MkdirAll(objDir, 0700)
			file, err := os.Create(objFile)
			if err != nil {
				utils.ExitWithError("Could not write tree of index using name %q in directory %q", objFile, objDir)
			}

			defer file.Close()

			var b bytes.Buffer
			compressor := zlib.NewWriter(&b)
			compressor.Write([]byte(treeString))
			compressor.Close()

			err = os.WriteFile(objFile, b.Bytes(), 0700)
			if err != nil {
				utils.ExitWithError("Could not write compressed contents of index to %v", objFile)
			}

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
			commit := got.CreateCommit(id, parentId, s[0])

			// Post Commit:
			// 1. Clear index
			index.Clear()

			// 2. Update hash pointed at in HEAD
			if utils.Exists(path) {
				if err := os.Truncate(path, 0); err != nil {
					utils.ExitWithError("Could not clear ref file %v: %v", path, err)
				}
			}

			if err := os.WriteFile(path, []byte(commit.Id), READ_WRITE_PERM); err != nil {
				utils.ExitWithError("Could not write commit id %v to %v file: %v", commit.Id, path, err)
			}
		},
	}
}

func CheckoutCommand() *Command {
	return &Command{
		Name:  "checkout",
		Short: "Checkout a specific commit",
		Long:  "Checkout a specific commit, causing the working directory to revert to the state contained in the commit",
		Run: func(s []string) {
			if len(s) != 1 {
				utils.ExitWithError("You can only pass exactly one argument [commit hash] to this command")
			}

			id := s[0]

			if len(id) < 6 {
				utils.ExitWithError("You must provid at least 6 characters of the commit ID")
			}

			// 1. Search for commit object
			file, err := got.GetObjectFile(id)
			if err != nil {
				utils.ExitWithError("Error getting object file for checkout command: %v", err)
			}

			// 2. Decompress and restore working directory
			decompressor, err := zlib.NewReader(file)
			if err != nil {
				utils.ExitWithError("Could not create decompressor %v: ", err)
			}
			decompressor.Close()
			var out bytes.Buffer
			io.Copy(&out, decompressor)

			lines := strings.Split(out.String(), "\n")
			for _, line := range lines {
				fmt.Println(line + "\n")
			}

			// 3. Update HEAD to point at provided commit
		},
	}
}
