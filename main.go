package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const GOT_REPO = ".got"
const READ_WRITE_PERM = 0700

func main() {
	hashObject("main.go", true)
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

func Execute() {
	if len(os.Args) < 2 {
		exitWithError("Not enough arguments")
	}

	var cmd *Command
	subCmd := os.Args[1]

	switch subCmd {
	case "init":
		cmd = InitCommand()
	default:
		cmd = UnknownCommand(subCmd)
	}

	cmd.Run(os.Args[1:])
}

// got library
func hashObject(obj string, write bool) string {
	if !exists(obj) {
		exitWithError("Cannot hash %q. Object doesn't exist", obj)
	}

	objType := "blob"

	if isDir(obj) {
		objType = "tree"
	}

	info, err := os.Stat(obj)
	if err != nil {
		exitWithError("Could not get file size for hash of %q", obj)
	}

	toHash := fmt.Sprintf("%v %d\u0000", objType, info.Size())
	hasher := sha1.New()
	hasher.Write(hasher.Sum([]byte(toHash)))
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
	}

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
