package got

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Index struct {
	entries []indexEntry
}

type indexEntry struct {
	Id     string
	Name   string
	IsDir  bool
	Status string
}

func (i *Index) Entries() []indexEntry {
	return i.entries
}

func (i *Index) Clear() error {
	if err := os.Truncate(config.IndexFile, 0); err != nil {
		return err
	}
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
		_, err := os.Stat(fName)
		if err != nil {
			return err
		}

		blobId, _, err := formatHexId(fName, BLOB)
		if err != nil {
			return err
		}

		found, index := i.IncludesFile(path)
		if found {
			i.entries[index].Id = blobId
			return nil
		}

		entry := indexEntry{
			Id:     blobId,
			Name:   path,
			IsDir:  false,
			Status: STATUS_ADD, // Need to implement logic to determine status
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

	// Currently removing file from index, later update status instead
	i.entries = append(i.entries[:idx], i.entries[idx+1:]...)
	return true, nil
}

func (i *Index) Save() error {
	if _, err := os.Stat(config.IndexFile); err != nil {
		return err
	}

	if err := os.Truncate(config.IndexFile, 0); err != nil {
		return err
	}

	contents := ""
	for _, entry := range i.entries {
		contents += fmt.Sprintf("%v %v %v\n", entry.Status, entry.Id, entry.Name)
	}

	return os.WriteFile(config.IndexFile, []byte(contents), 0700)
}

func (i *Index) Length() int {
	return len(i.entries)
}

func (i *Index) Commit(msg string) error {
	entryNames := make([]string, i.Length())

	for i, entry := range i.Entries() {
		entryNames[i] = entry.Name
	}

	slices.Sort(entryNames)

	var tree string
	for _, name := range entryNames {
		id, objectType := WriteObject(name)
		tree += fmt.Sprintf("%v %v %v %v\n", 100644, objectType, id, name)
	}

	id, treeString, err := formatHexId(tree, TREE)
	if err != nil {
		return err
	}

	objDir := filepath.Join(config.ObjectDB, id[:2])
	objFile := filepath.Join(objDir, id[2:])

	if err = os.MkdirAll(objDir, 0700); err != nil {
		return err
	}

	file, err := os.Create(objFile)
	if err != nil {
		return err
	}

	defer file.Close()

	var b bytes.Buffer
	compressor := zlib.NewWriter(&b)

	if _, err = compressor.Write([]byte(treeString)); err != nil {
		return err
	}

	compressor.Close()

	if err = os.WriteFile(objFile, b.Bytes(), 0700); err != nil {
		return err
	}

	// 2. get parent commit, if Exists
	headRef, err := os.ReadFile(config.HeadFile)
	if err != nil {
		return err
	}

	ref := strings.Split(string(headRef), ":")
	pathBits := strings.Split(strings.TrimSpace(ref[1]), "/")
	pathToRef := append([]string{config.Repo}, pathBits...)
	path := filepath.Join(pathToRef...)

	if _, err = os.Stat(path); err != nil {
		return err
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	parentId := string(contents)

	// 3. Create commit
	commit, err := newCommit(id, parentId, msg)
	if err != nil {
		return err
	}

	// Post Commit:
	// 1. Clear i
	if err = i.Clear(); err != nil {
		return err
	}

	// 2. Update hash pointed at in HEAD
	if err := os.Truncate(path, 0); err != nil {
		return err
	}

	rw := fs.FileMode(0666)
	if err := os.WriteFile(path, []byte(commit.Id), rw); err != nil {
		return fmt.Errorf("Could not write commit id %v to %v file: %v", commit.Id, path, err)
	}

	return nil
}

// TODO: Could do with work
func GetIndex() (Index, error) {
	index := Index{}

	_, err := os.Stat(config.IndexFile)
	if err != nil {
		return index, err
	}

	indexFile, err := os.Open(config.IndexFile)
	if err != nil {
		return index, err
	}

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
