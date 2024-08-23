package got

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	Repo             filePath = ".got"
	IndexFile        filePath = "index"
	RefsDir          filePath = "refs"
	RefHeadsDir      filePath = "heads"
	RefHeadsMainFile filePath = "main"
	ObjectsDir       filePath = "objects"
	HeadFile         filePath = "HEAD"
	ConfigFile       filePath = "config"
)

func getRepoPath() (filePath, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory: %w", err)
	}
	return filepath.Join(wd, Repo), nil
}

func getIndexPath() (filePath, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory for index: %w", err)
	}
	return filepath.Join(workingDir, Repo, IndexFile), nil
}

func getHeadPath() (filePath, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory for head: %w", err)
	}
	return filepath.Join(workingDir, Repo, HeadFile), nil
}

func getConfigPath() (filePath, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory for config: %w", err)
	}
	return filepath.Join(workingDir, Repo, ConfigFile), nil
}

func getRefsDirPath() (filePath, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory for refs dir: %w", err)
	}
	return filepath.Join(workingDir, Repo, RefsDir), nil
}

func getRefHeadsDirPath() (filePath, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory for heads dir: %w", err)
	}
	return filepath.Join(workingDir, Repo, RefsDir, RefHeadsDir), nil
}

func getRefHeadsMainFilePath() (filePath, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory for refs head main: %w", err)
	}
	return filepath.Join(workingDir, Repo, RefsDir, RefHeadsDir, RefHeadsMainFile), nil
}

func getObjectsDirPath() (filePath, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory for objects dir: %w", err)
	}
	return filepath.Join(workingDir, Repo, ObjectsDir), nil
}
