package got

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	BLOB          = "blob"
	TREE          = "tree"
	COMMIT        = "commit"
	STATUS_ADD    = "A"
	STATUS_MODIFY = "M"
	STATUS_DELETE = "D"
)

var (
	config       *Config
	stagingIndex Index
)

type Config struct {
	Repo      string
	RefsDir   string
	HeadsDir  string
	HeadFile  string
	IndexFile string
	ObjectDB  string
}

type GotObject interface {
	HexId() string
}

type object struct {
	Id   string
	Type string
}

type Blob struct {
	object
}

type Tree struct {
	object
}

type Commit struct {
	object
	Author    string
	CreatedAt time.Time
	Message   string
	ParentId  string
	TreeId    string
}

func (o object) HexId() string {
	return o.Id
}

func GetConfig() Config {
	if config == nil {
		return Config{
			Repo:      ".got",
			RefsDir:   ".got/refs",
			HeadsDir:  ".got/refs/heads",
			HeadFile:  ".got/refs/heads/HEAD",
			IndexFile: ".got/index",
			ObjectDB:  ".got/objects",
		}
	}
	return *config
}

func newBlob(id string) *Blob {
	return &Blob{object{Id: id, Type: BLOB}}
}

func newTree(id string) *Tree {
	return &Tree{object{Id: id, Type: TREE}}
}

func GetObjectFile(id string) (*os.File, error) {
	db := filepath.Join(config.ObjectDB)
	objects, err := os.ReadDir(db)
	if err != nil {
		return nil, err
	}

	for _, dir := range objects {
		if dir.Name() == id[:2] {
			dPath := filepath.Join(db, dir.Name())
			files, err := os.ReadDir(dPath)
			if err != nil {
				return nil, err
			}

			for _, file := range files {
				if strings.HasPrefix(file.Name(), id[2:]) {
					fPath := filepath.Join(dPath, file.Name())
					return os.Open(fPath)
				}
			}
		}
	}

	return nil, fmt.Errorf("Could not find object file for %q\n", id)
}

func WriteObject(obj string) (GotObject, error) {
	info, err := os.Stat(obj)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		t, err := writeTree(obj)
		if err != nil {
			return nil, err
		}

		return *t, nil
	}

	b, err := writeBlob(obj)
	if err != nil {
		return nil, err
	}

	return *b, nil
}

func writeBlob(fileName string) (*Blob, error) {
	id, blobString, err := formatHexId(fileName, BLOB)
	if err != nil {
		return nil, err
	}

	objDir := filepath.Join(config.ObjectDB, id[:2])
	objFile := filepath.Join(objDir, id[2:])

	os.MkdirAll(objDir, 0700)
	file, err := os.Create(objFile)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	var b bytes.Buffer
	compressor := zlib.NewWriter(&b)
	compressor.Write([]byte(blobString))
	compressor.Close()

	err = os.WriteFile(objFile, b.Bytes(), 0700)
	if err != nil {
		return nil, fmt.Errorf("Could not write compressed contents of %v to %v", fileName, objFile)
	}

	return newBlob(id), nil
}

func writeTree(path string) (*Tree, error) {
	_, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	var content string
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		filePath := filepath.Join(path, file.Name())
		if file.IsDir() {
			tree, err := writeTree(filePath)
			if err != nil {
				return nil, err
			}
			content += fmt.Sprintf("%v tree %v %v\n", 100644, tree.Id, file.Name())
		} else {
			blob, err := writeBlob(filePath)
			if err != nil {
				return nil, err
			}
			content += fmt.Sprintf("%v blob %v %v\n", 100644, blob.Id, file.Name())
		}

	}

	id, treeString, err := formatHexId(content, TREE)
	if err != nil {
		return nil, err
	}

	objDir := filepath.Join(config.ObjectDB, id[:2])
	objFile := filepath.Join(objDir, id[2:])

	os.MkdirAll(objDir, 0700)
	file, err := os.Create(objFile)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	var b bytes.Buffer
	compressor := zlib.NewWriter(&b)
	compressor.Write([]byte(treeString))
	compressor.Close()

	err = os.WriteFile(objFile, b.Bytes(), 0700)
	if err != nil {
		return nil, err
	}

	return newTree(id), nil
}

func newCommit(tree string, parentId string, msg string) (*Commit, error) {
	parentListing := ""
	if parentId != "" {
		parentListing = fmt.Sprintf("parent %v", parentId)
	}

	committer := "ljpurcell" // TODO work on Got config

	data := fmt.Sprintf("tree %v\n%v\ncommiter %v\n\n%v", tree, parentListing, committer, msg)

	id, commitString, err := formatHexId(data, COMMIT)
	if err != nil {
		return nil, err
	}

	objDir := filepath.Join(config.ObjectDB, id[:2])
	objFile := filepath.Join(objDir, id[2:])

	os.MkdirAll(objDir, 0700)
	file, err := os.Create(objFile)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	var b bytes.Buffer
	compressor := zlib.NewWriter(&b)
	compressor.Write([]byte(commitString))
	compressor.Close()

	err = os.WriteFile(objFile, b.Bytes(), 0700)
	if err != nil {
		return nil, err
	}

	return &Commit{
		object: object{
			Id:   id,
			Type: COMMIT,
		},
		Author:    "ljpurcell",
		CreatedAt: time.Now(),
		Message:   msg,
		ParentId:  "",
		TreeId:    tree,
	}, nil
}

func formatHexId(obj string, objType string) (id string, objString string, err error) {
	size, content := len(obj), obj
	if objType == BLOB {
		fileContents, err := os.ReadFile(obj)
		if err != nil {
			return "", "", err
		}
		info, err := os.Stat(obj)
		if err != nil {
			return "", "", err
		}
		size = int(info.Size())
		content = string(fileContents)
	}

	objString = fmt.Sprintf("%v %d\n%v", objType, size, content)
	hasher := sha1.New()
	hasher.Write([]byte(objString))
	id = hex.EncodeToString(hasher.Sum(nil))

	return id, objString, nil
}
