package got

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	cfg "github.com/ljpurcell/got/internal/config"
	"github.com/ljpurcell/got/internal/utils"
)

type Commit struct {
	Id        string
	Author    string
	CreatedAt time.Time
	Message   string
	ParentId  string
	TreeId    string
}

func HashObject(obj string) (id string, objectType string) {
	if utils.IsDir(obj) {
		id = hashTree(obj)
		objectType = "tree"
		return
	}

	id = hashBlob(obj)
	objectType = "blob"
	return
}

func hashBlob(fileName string) string {
	if !utils.Exists(fileName) {
		utils.ExitWithError("Cannot hash %q. Object doesn't exist", fileName)
	}

	if utils.IsDir(fileName) {
		utils.ExitWithError("Cannot call hash blob on %q. Object is a directory", fileName)
	}

	id, _ := formatHexId(fileName, "blob")

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

	id, treeString := formatHexId(tree, "tree")

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

func formatHexId(obj string, objType string) (id string, objectString string) {

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

	objectString = fmt.Sprintf("%v %d\u0000%v", objType, size, content)
	hasher := sha1.New()
	hasher.Write([]byte(objectString))
	id = hex.EncodeToString(hasher.Sum(nil))
	return
}
