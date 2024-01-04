package main

/**
* General utility functions
*/

import (
	"fmt"
	"os"
)

func Exists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}

		ExitWithError("Could not check if file %v exists: %v", path, err)
	}

	return true
}

func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		ExitWithError("Could not check if file %v is directory: %v", path, err)
	}

	return info.IsDir()
}

func ExitWithError(msg string, args ...interface{}) {
	var output string
	if len(args) > 0 {
		output = fmt.Sprintf(msg+"\n", args...)
	} else {
		output = fmt.Sprintf(msg + "\n")
	}

	fmt.Fprintf(os.Stderr, output)
	os.Exit(1)
}
