package got

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type status = string

const (
	STATUS_ADD    = "A"
	STATUS_MODIFY = "M"
	STATUS_DELETE = "D"
)

type storer interface {
	io.ReadWriter
	Truncate(int64) error
}

type Index struct {
	entries []indexEntry
	storage storer
}

type indexEntry struct {
	Id     string
	Name   filePath
	IsDir  bool
	Status status
}

func (i *Index) Entries() []indexEntry {
	return i.entries
}

func (i *Index) Clear() error {
	if err := i.storage.Truncate(0); err != nil {
		return err
	}

	i.entries = []indexEntry{}
	return nil
}

func (i *Index) IncludesFile(file string) (bool, int) {
	for idx, entry := range i.entries {
		if entry.Name == file {
			return true, idx
		}
	}

	return false, -1
}

func (i *Index) UpdateOrAddEntry(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}

	files := []string{path}

	if fi.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		files = []string{}

		for _, entry := range entries {
			nestedPath := filepath.Join(path, entry.Name())
			if entry.IsDir() {
				if err = i.UpdateOrAddEntry(nestedPath); err != nil {
					return err
				}
			} else {
				files = append(files, nestedPath)
			}
		}
	}

	for _, fName := range files {
		if _, err := os.Stat(fName); err != nil {
			return err
		}

		blobId, _, err := formatHexId(fName, BLOB)
		if err != nil {
			return err
		}

		found, entryIndex := i.IncludesFile(path)
		if found {
			i.entries[entryIndex].Id = blobId
			return nil
		}

		parent, err := getHeadCommit()
		if err != nil {
			return fmt.Errorf("could not get head commit: %s", err)
		}

		status := STATUS_ADD

		if parent != nil {
			if _, ok := parent.Entries[fName]; ok {
				status = STATUS_MODIFY
			}
		}

		entry := indexEntry{
			Id:     blobId,
			Name:   path,
			IsDir:  false,
			Status: status,
		}

		i.entries = append(i.entries, entry)
	}

	return nil
}

func (i *Index) RemoveFile(file string) (removed bool, err error) {
	err = os.Remove(file)
	if err != nil {
		return false, err
	}

	found, idx := i.IncludesFile(file)
	if !found {
		return false, nil
	}

	// TODO: currently removing file from index, later update status instead
	i.entries = append(i.entries[:idx], i.entries[idx+1:]...)
	return true, nil
}

func (i *Index) Save() error {
	indexPath, err := getIndexPath()
	if err != nil {
		return fmt.Errorf("could not get index path: %w", err)
	}
	if _, err := os.Stat(indexPath); err != nil {
		return err
	}

	if err := os.Truncate(indexPath, 0); err != nil {
		return err
	}

	contents := ""
	for _, entry := range i.entries {
		contents += fmt.Sprintf("%v %v %v\n", entry.Status, entry.Id, entry.Name)
	}

	return os.WriteFile(indexPath, []byte(contents), 0700)
}

func (i *Index) Length() int {
	return len(i.entries)
}

func (i *Index) Commit(msg string) error {
	cb := newCommitBuilder()
	cb.message(msg)

	if err := cb.entries(i.Entries()); err != nil {
		return fmt.Errorf("commit builder entries method: %w", err)
	}

	if err := cb.setParent(); err != nil {
		return fmt.Errorf("commit builder set parent method: %w", err)
	}

	commit, err := cb.build()
	if err != nil {
		return fmt.Errorf("commit builder build method: %w", err)
	}

	if err = i.Clear(); err != nil {
		return err
	}

	// 2. Update hash pointed at by main in ref file
	refsHeadMainPath, err := getRefHeadsMainFilePath()
	if err != nil {
		return fmt.Errorf("could not get ref head main file path: %w", err)
	}
	if err := os.Truncate(refsHeadMainPath, 0); err != nil {
		return err
	}

	rw := fs.FileMode(0666)
	if err := os.WriteFile(refsHeadMainPath, []byte(commit.Id), rw); err != nil {
		return fmt.Errorf("Could not write commit id %v to %v file: %v", commit.Id, refsHeadMainPath, err)
	}

	return nil
}

// TODO: Could do with work
func GetIndex() (Index, error) {
	indexPath, err := getIndexPath()
	if err != nil {
		return Index{}, fmt.Errorf("could not get index path: %w", err)
	}
	indexFile, err := os.Open(indexPath)
	if err != nil {
		return Index{}, fmt.Errorf("could not open index file: %w", err)
	}
	defer indexFile.Close()

	index := Index{}
	scanner := bufio.NewScanner(indexFile)
	for scanner.Scan() {
		entryParts := strings.Split(scanner.Text(), " ")
		entry := indexEntry{
			Id:     entryParts[1],
			Name:   entryParts[2],
			Status: entryParts[0],
		}

		index.entries = append(index.entries, entry)
	}

	return index, nil
}
