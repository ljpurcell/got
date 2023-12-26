package main

import (
	"fmt"
	"os"
)

const GOT_REPO = ".got"

func main() {
	Execute()
}

type Command struct {
	Name  string
	Short string
	Long  string
	Help  string
	Run   func([]string) error
}

func InitCommand() *Command {
	return &Command{
		Name:  "init",
		Short: "Initialises a got repository",
		Long:  "Initialises a got repository with a hidden .got file to hold data",
		Run: func(s []string) error {
			if exists(GOT_REPO) {
				if isDir(GOT_REPO) {
					exitWithError("%q repo already initialised", GOT_REPO)
				}
				exitWithError("File named %q already exists", GOT_REPO)
			}

            os.Mkdir(GOT_REPO, 0700)
			return nil
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
	}

	cmd.Run(os.Args[1:])
}

// utils
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
