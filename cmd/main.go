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
	Run   func([]string) error
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

	if err := cmd.Run(os.Args[2:]); err != nil {
		return fmt.Errorf("%s command error: %w", subCmd, err)
	}

	return nil
}

func UnknownCommand(cmd string) *Command {
	return &Command{
		Name:  "unknown",
		Short: "Got did not recognise the subcommand",
		Long:  "Got did not recognise the subcommand; try running \"got --help\" for more",
		Run: func(_ []string) error {
			return fmt.Errorf("did not recognise subcommand %q", cmd)
		},
	}
}

func InitCommand() *Command {
	return &Command{
		Name:  "init",
		Short: "Initialises a got repository",
		Long:  "Initialises a got repository with a hidden .got file to hold data",
		Run: func(args []string) error {
			config := got.GetConfig()
			_, err := os.Stat(config.Repo)
			if err != nil {
				return err
			}

			rw := fs.FileMode(0666)

			if err = os.MkdirAll(config.HeadsDir, rw); err != nil {
				return fmt.Errorf("could not create heads directory path: %w", err)
			}

			head, err := os.Create(config.HeadFile)
			if err != nil {
				return fmt.Errorf("could not create HEAD file: %w", err)
			}
			defer head.Close()

			if err = os.WriteFile(config.HeadFile, []byte("ref: refs/heads/main"), rw); err != nil {
				return fmt.Errorf("could not write to HEAD file: %w", err)
			}

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
		Run: func(args []string) error {
			if len(args) < 1 {
				return errors.New("not enough arguments")
			}

			index, err := got.GetIndex()
			if err != nil {
				return err
			}

			for _, file := range args {
				if err = index.UpdateOrAddEntry(file); err != nil {
					return err
				}
			}

			return index.Save()
		},
	}
}

func RemoveCommand() *Command {
	return &Command{
		Name:  "remove",
		Short: "Remove objects from the working directory",
		Long:  "Remove files or directories from the index (staging area)",
		Run: func(args []string) error {
			if len(args) < 1 {
				return errors.New("not enough arguments")
			}

			index, err := got.GetIndex()
			if err != nil {
				return err
			}

			for _, file := range args {
				if _, err = index.RemoveFile(file); err != nil {
					return fmt.Errorf("could not remove file %s: %w", file, err)
				}
			}

			return index.Save()
		},
	}
}

func CommitCommand() *Command {
	return &Command{
		Name:  "commit",
		Short: "Commit the current index",
		Long:  "Create a commit (snapshot) of the current state of the objects listed in the index",
		Run: func(args []string) error {
			if len(args) != 1 {
				return errors.New("you can only pass exactly one argument [commit message] to this command")
			}

			index, err := got.GetIndex()
			if err != nil {
				return errors.New("could not get index file")
			}

			if index.Length() == 0 {
				return errors.New("index file empty")
			}

			return index.Commit(args[0])
		},
	}
}

func CheckoutCommand() *Command {
	return &Command{
		Name:  "checkout",
		Short: "Checkout a specific commit",
		Long:  "Checkout a specific commit, causing the working directory to revert to the state contained in the commit",
		Run: func(args []string) error {
			if len(args) != 1 {
				return errors.New("you can only pass exactly one argument [commit hash] to this command")
			}

			id := args[0]

			if len(id) < 6 {
				return errors.New("you must provid at least 6 characters of the commit ID")
			}

			file, err := got.GetObjectFile(id)
			if err != nil {
				return fmt.Errorf("error getting object file: %w", err)
			}

			decompressor, err := zlib.NewReader(file)
			if err != nil {
				return fmt.Errorf("could not create decompressor: %w ", err)
			}

			decompressor.Close()
			var out bytes.Buffer
			if _, err = io.Copy(&out, decompressor); err != nil {
				return err
			}

			lines := strings.Split(out.String(), "\n")
			for _, line := range lines {
				fmt.Println(line + "\n")
			}

			// 3. Update HEAD to point at provided commit

			return nil
		},
	}
}
