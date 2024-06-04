package main

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	got "github.com/ljpurcell/got/internal"
)

type Command struct {
	Name  string
	Short string
	Long  string
	Help  string
	Run   func(got.Config, []string) error
}

func main() {
	if err := execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute() error {
	if len(os.Args) < 2 {
		return errors.New("not enough arguments")
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

	config := got.GetConfig()

	if err := cmd.Run(config, os.Args[2:]); err != nil {
		return err
	}

	return nil
}

func UnknownCommand(cmd string) *Command {
	return &Command{
		Name:  "unknown",
		Short: "Got did not recognise the subcommand",
		Long:  "Got did not recognise the subcommand; try running \"got --help\" for more",
		Run: func(_ got.Config, _ []string) error {
			return fmt.Errorf("Got did not recognise the subcommand %q", cmd)
		},
	}
}

func InitCommand() *Command {
	return &Command{
		Name:  "init",
		Short: "Initialises a got repository",
		Long:  "Initialises a got repository with a hidden .got file to hold data",
		Run: func(config got.Config, args []string) error {
			_, err := os.Stat(config.Repo)
			if err != nil {
				return err
			}

			rw := fs.FileMode(0666)

			os.MkdirAll(config.HeadsDir, rw)

			head, err := os.Create(config.HeadFile)
			if err != nil {
				return fmt.Errorf("could not create HEAD file: %w", err)
			}
			defer head.Close()

			os.WriteFile(config.HeadFile, []byte("ref: refs/heads/main"), rw)

			fmt.Fprintf(os.Stdout, "Initialised an empty got repository\n")
			return nil
		},
	}
}

func AddCommand() *Command {
	return &Command{
		Name:  "add",
		Short: "Add objects to the index",
		Long:  "Add files or directories to the index (staging area)",
		Run: func(config got.Config, args []string) error {
			if len(args) < 1 {
				return errors.New("not enough arguments")
			}

			index, err := got.GetIndex()
			if err != nil {
				return err
			}

			for _, file := range args {
				index.UpdateOrAddEntry(file)
			}

			index.Save()
			return nil
		},
	}
}

func RemoveCommand() *Command {
	return &Command{
		Name:  "remove",
		Short: "Remove objects from the working directory",
		Long:  "Remove files or directories from the index (staging area)",
		Run: func(config got.Config, args []string) error {
			if len(args) < 1 {
				return errors.New("not enough arguments")
			}

			index, err := got.GetIndex()
			if err != nil {
				return err
			}

			for _, file := range args {
				index.RemoveFile(file)
			}

			index.Save()
			return nil
		},
	}
}

func CommitCommand() *Command {
	return &Command{
		Name:  "commit",
		Short: "Commit the current index",
		Long:  "Create a commit (snapshot) of the current state of the objects listed in the index",
		Run: func(config got.Config, args []string) error {
			if len(args) != 1 {
				errors.New("You can only pass exactly one argument [commit message] to this command")
			}

			msg := args[0]

			// 1. Generate an up to date tree hash for all listings in index
			index, err := got.GetIndex()
			if err != nil {
				return err
			}

			if index.Length() == 0 {
				return errors.New("Index file empty. Nothing staged to commit")
			}

			index.Commit(msg)

			return nil
		},
	}
}

func CheckoutCommand() *Command {
	return &Command{
		Name:  "checkout",
		Short: "Checkout a specific commit",
		Long:  "Checkout a specific commit, causing the working directory to revert to the state contained in the commit",
		Run: func(config got.Config, args []string) error {
			if len(args) != 1 {
				return errors.New("You can only pass exactly one argument [commit hash] to this command")
			}

			id := args[0]

			if len(id) < 6 {
				return errors.New("You must provid at least 6 characters of the commit ID")
			}

			// 1. Search for commit object
			file, err := got.GetObjectFile(id)
			if err != nil {
				return fmt.Errorf("Error getting object file for checkout command: %w", err)
			}

			// 2. Decompress and restore working directory
			decompressor, err := zlib.NewReader(file)
			if err != nil {
				return fmt.Errorf("Could not create decompressor %v: ", err)
			}
			decompressor.Close()
			var out bytes.Buffer
			io.Copy(&out, decompressor)

			lines := strings.Split(out.String(), "\n")
			for _, line := range lines {
				fmt.Println(line + "\n")
			}

			// 3. Update HEAD to point at provided commit

			return nil
		},
	}
}
