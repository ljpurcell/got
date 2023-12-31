package got

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	cfg "github.com/ljpurcell/got/internal/config"
	"github.com/ljpurcell/got/internal/utils"
)

const (
	BLOB   = "blob"
	TREE   = "tree"
	COMMTI = "commit"
)

type Commit struct {
	Id        string
	Author    string
	CreatedAt time.Time
	Message   string
	ParentId  string
	TreeId    string
}

type Index struct {
    entries []indexEntry
}

type indexEntry struct {
	Id     string
	File   string
	Status string
}

func HashObject(obj string) (id string, objectType string) {
	if utils.IsDir(obj) {
		id = hashTree(obj)
		objectType = TREE
		return
	}

	id = hashBlob(obj)
	objectType = BLOB
	return
}

func hashBlob(fileName string) string {
	if !utils.Exists(fileName) {
		utils.ExitWithError("Cannot hash %q. Object doesn't exist", fileName)
	}

	if utils.IsDir(fileName) {
		utils.ExitWithError("Cannot call hash blob on %q. Object is a directory", fileName)
	}

	id, _ := formatHexId(fileName, BLOB)

	objDir := filepath.Join(cfg.GOT_REPO, "objects", id[:2])
	objFile := filepath.Join(objDir, id[2:])

	os.MkdirAll(objDir, 0700)
	file, err := os.Create(objFile)
	if err != nil {
		utils.ExitWithError("Could not write object %q using name %q in directory %q", fileName, objFile, objDir)
	}

	defer file.Close()

	fileContents, err := os.ReadFile(fileName)
	if err != nil {
		utils.ExitWithError("Could not read contents from file %v for compression", fileName)
	}

	var b bytes.Buffer
	compressor := zlib.NewWriter(&b)
	compressor.Write([]byte(fileContents))
	compressor.Close()

	err = os.WriteFile(objFile, b.Bytes(), 0700)
	if err != nil {
		utils.ExitWithError("Could not write compressed contents of %v to %v", fileName, objFile)
	}

	return id
}

func hashTree(dir string) string {
	if !utils.Exists(dir) {
		utils.ExitWithError("Cannot hash %q. Object doesn't exist", dir)
	}

	if !utils.IsDir(dir) {
		utils.ExitWithError("Cannot call hash tree on %q. Object is not a directory", dir)
	}

	var tree string
	files, err := os.ReadDir(dir)
	if err != nil {
		utils.ExitWithError("Could not read files for %v: %v", dir, err)
	}

	for _, file := range files {
		filePath := filepath.Join(dir, file.Name())
		if file.IsDir() {
			treeId := hashTree(filePath)
			tree += fmt.Sprintf("%v tree %v %v\n", 100644, treeId, file.Name())
		} else {
			blobId := hashBlob(filePath)
			tree += fmt.Sprintf("%v blob %v %v\n", 100644, blobId, file.Name())
		}

	}

	id, treeString := formatHexId(tree, TREE)

	objDir := filepath.Join(cfg.GOT_REPO, "objects", id[:2])
	objFile := filepath.Join(objDir, id[2:])

	os.MkdirAll(objDir, 0700)
	file, err := os.Create(objFile)
	if err != nil {
		utils.ExitWithError("Could not write object %q (tree) using name %q in directory %q", dir, objFile, objDir)
	}

	defer file.Close()

	var b bytes.Buffer
	compressor := zlib.NewWriter(&b)
	compressor.Write([]byte(treeString))
	compressor.Close()

	err = os.WriteFile(objFile, b.Bytes(), 0700)
	if err != nil {
		utils.ExitWithError("Could not write compressed contents of %v to %v", dir, objFile)
	}

	return id
}

func CreateCommit(tree string, parentId string, msg string) *Commit {
	parentListing := ""
	if parentId != "" {
		parentListing = fmt.Sprintf("parent %v", parentId)
	}

	committer := "ljpurcell" // TODO work on Got config

	data := fmt.Sprintf("tree %v\n%v\ncommiter %v\n\n%v", tree, parentListing, committer, msg)

	id, commitString := formatHexId(data, "commit")

	objDir := filepath.Join(cfg.GOT_REPO, "objects", id[:2])
	objFile := filepath.Join(objDir, id[2:])

	os.MkdirAll(objDir, 0700)
	file, err := os.Create(objFile)
	if err != nil {
		utils.ExitWithError("Could not write object (commit) using name %q in directory %q", objFile, objDir)
	}

	defer file.Close()

	var b bytes.Buffer
	compressor := zlib.NewWriter(&b)
	compressor.Write([]byte(commitString))
	compressor.Close()

	err = os.WriteFile(objFile, b.Bytes(), 0700)
	if err != nil {
		utils.ExitWithError("Could not write compressed contents of commit with message %q to %v", msg, objFile)
	}

	return &Commit{
		Id:        id,
		Author:    "ljpurcell",
		CreatedAt: time.Now(),
		Message:   msg,
		ParentId:  "",
		TreeId:    tree,
	}
}

func formatHexId(obj string, objType string) (id string, objString string) {

	size, content := len(obj), obj
	if objType == "blob" {
		fileContents, err := os.ReadFile(obj)
		if err != nil {
			utils.ExitWithError("Could not read file %v: %v", obj, err)
		}
		info, err := os.Stat(obj)
		if err != nil {
			utils.ExitWithError("Could not get file size for hash of %q", obj)
		}
		size = int(info.Size())
		content = string(fileContents)
	}

	objString = fmt.Sprintf("%v %d\u0000%v", objType, size, content)
	hasher := sha1.New()
	hasher.Write([]byte(objString))
	id = hex.EncodeToString(hasher.Sum(nil))
	return
}

func GetIndexEntries() Index {
	index := Index{}

	path := filepath.Join(cfg.GOT_REPO, cfg.INDEX_FILE)
	if !utils.Exists(path) {
		return index
	}

	indexFile, err := os.Open(path)
	if err != nil {
		utils.ExitWithError("Could not open index file: %v", err)
	}

	scanner := bufio.NewScanner(indexFile)
	for scanner.Scan() {
		entryParts := strings.Split(scanner.Text(), " ")
		entry := indexEntry{
			Id:     entryParts[1],
			File:   entryParts[2],
			Status: entryParts[0],
		}

		index.entries = append(*&index.entries, entry)
	}

	return index
}

func (i *Index) IncludesFile(file string) (bool, int) {
    for idx, entry := range *&i.entries {
        if entry.File == file {
            return true, idx
        }
    }

    return false, -1
}

func (i *Index) UpdateOrAdd(entry indexEntry) {
    found, index := i.IncludesFile(entry.File)
    if found {
        i.entries[index] = entry
        return
    }

    i.entries = append(i.entries, entry)
}

func (i *Index) Remove(file string) bool {
    found, idx := i.IncludesFile(file)
    if found {
        i.entries = append(i.entries[:idx], i.entries[idx+1:]...)
        return true
    }

    return false
}

func (i *Index) Save() {
	path := filepath.Join(cfg.GOT_REPO, cfg.INDEX_FILE)
	if utils.Exists(path) {
        os.Truncate(path, 0)
	}

    contents := ""
    for _, entry := range *&i.entries {
        contents += fmt.Sprintf("%v %v %v", entry.Status, entry.Id, entry.File)
    }

    fmt.Println(contents)
    os.WriteFile(path, []byte(contents), 0700)
}
