package main

import (
	"fmt"
	"os"
)

func main() {
    Execute()
}

type Command struct {
    Name string
    Short string
    Long string
    Help string
    Run func([]string) error
}

func InitCommand() *Command {
    return &Command{
        Name: "init",
        Short: "Initialises a got repository",
        Long: "Initialises a got repository with a hidden .got file to hold data",
        Run: func(s []string) error {
            fmt.Printf("Initalising...\n")
            return nil
        },
    }

}

func Execute() {
    if (len(os.Args) < 2) {
        fmt.Fprintf(os.Stderr, "Not enough arguments\n")
        // Print HELP from main command?
        os.Exit(1)
    }

    var cmd *Command
    subCmd := os.Args[1]

    switch (subCmd) {
    case "init":
        cmd = InitCommand()
    }

    cmd.Run(os.Args[1:])
}
