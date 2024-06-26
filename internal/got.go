package got

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type (
	filePath   = string
	id         = string
	objectPath = string
	objectType = string
)

const (
	BLOB   objectType = "blob"
	TREE   objectType = "tree"
	COMMIT objectType = "commit"
)

var (
	config       *Config
	stagingIndex Index
)

type Config struct {
	Repo             filePath
	RefsDir          filePath
	RefsHeadMainFile filePath
	HeadFile         filePath
	IndexFile        filePath
	ObjectDB         filePath
}

type GotObject interface {
	HexId() id
}

type object struct {
	Id   id
	Type objectType
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
	Parent    id
	Tree      id
	Entries   map[filePath]id
}

func (o object) HexId() string {
	return o.Id
}

type commitBuilder struct {
	commit *Commit
}

func (cb *commitBuilder) entries(entries []indexEntry) error {
	cb.commit.Entries = make(map[filePath]id, len(entries))

	entryNamesForTree := make([]string, 0, len(entries))

	for i, entry := range entries {
		cb.commit.Entries[entry.Name] = entry.Id
		entryNamesForTree[i] = entry.Name
	}

	slices.Sort(entryNamesForTree)

	var tree string

	for _, name := range entryNamesForTree {
		id, objectType := WriteObject(name)
		tree += fmt.Sprintf("%v %v %v %v\n", 100644, objectType, id, name)
	}

	treeId, treeString, err := formatHexId(tree, TREE)
	if err != nil {
		return err
	}

	cb.commit.Tree = treeId

	objDir := filepath.Join(config.ObjectDB, treeId[:2])
	objFile := filepath.Join(objDir, treeId[2:])

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

	return nil
}

func (cb *commitBuilder) message(msg string) {
	cb.commit.Message = msg
}

func (cb *commitBuilder) setParent() error {
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

	cb.commit.Parent = string(contents)
	return nil
}

func (cb *commitBuilder) build() (*Commit, error) {
	// TODO: Start here
	var parentListing string

	if cb.commit.Parent != "" {
		parentListing = fmt.Sprintf("parent %v", cb.commit.Parent)
	}

	committer := "ljpurcell" // TODO: work on Got config

	data := fmt.Sprintf("tree %v\n%v\ncommiter %v\n\n%v", cb.commit.Tree, parentListing, committer, cb.commit.Message)

	id, commitString, err := formatHexId(data, COMMIT)
	if err != nil {
		return nil, err
	}

	objDir := filepath.Join(config.ObjectDB, id[:2])
	objFile := filepath.Join(objDir, id[2:])

	if err = os.MkdirAll(objDir, 0700); err != nil {
		return nil, err
	}

	file, err := os.Create(objFile)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	var b bytes.Buffer
	compressor := zlib.NewWriter(&b)

	if _, err = compressor.Write([]byte(commitString)); err != nil {
		return nil, err
	}

	compressor.Close()

	if err = os.WriteFile(objFile, b.Bytes(), 0700); err != nil {
		return nil, err
	}

	return cb.commit, nil
}

func newCommitBuilder() *commitBuilder {
	return &commitBuilder{
		commit: &Commit{},
	}
}

func GetConfig() Config {
	if config == nil {
		return Config{
			Repo:             ".got",
			RefsDir:          ".got/refs",
			RefsHeadMainFile: ".got/refs/main",
			HeadFile:         ".got/refs/heads/HEAD",
			IndexFile:        ".got/index",
			ObjectDB:         ".got/objects",
		}
	}
	return *config
}

func newBlob(id id) *Blob {
	return &Blob{object{Id: id, Type: BLOB}}
}

func newTree(id id) *Tree {
	return &Tree{object{Id: id, Type: TREE}}
}

func Init() error {
	config := GetConfig()
	if _, err := os.Stat(config.Repo); err == nil {
		return fmt.Errorf("%s already exists", config.Repo)
	}

	rw := fs.FileMode(0666)

	if err := os.MkdirAll(config.RefsDir, rw); err != nil {
		return fmt.Errorf("could not create refs directory path: %w", err)
	}

	head, err := os.Create(config.HeadFile)
	if err != nil {
		return fmt.Errorf("could not create HEAD file: %w", err)
	}
	defer head.Close()

	if err = os.WriteFile(config.HeadFile, []byte("ref: refs/heads/main"), rw); err != nil {
		return fmt.Errorf("could not write to HEAD file: %w", err)
	}

	index, err := os.Create(config.IndexFile)
	if err != nil {
		return fmt.Errorf("could not create index file: %w", err)
	}
	defer index.Close()

	InitIndex(index)

	return nil
}

func GetObjectFile(id id) (*os.File, error) {
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

	return nil, fmt.Errorf("could not find object file for %q\n", id)
}

func WriteObject(op objectPath) (GotObject, error) {
	info, err := os.Stat(op)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		t, err := writeTree(op)
		if err != nil {
			return nil, err
		}

		return *t, nil
	}

	b, err := writeBlob(op)
	if err != nil {
		return nil, err
	}

	return *b, nil
}

func writeBlob(op objectPath) (*Blob, error) {
	id, blobString, err := formatHexId(op, BLOB)
	if err != nil {
		return nil, err
	}

	objDir := filepath.Join(config.ObjectDB, id[:2])
	objFile := filepath.Join(objDir, id[2:])

	if err = os.MkdirAll(objDir, 0700); err != nil {
		return nil, err
	}

	file, err := os.Create(objFile)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	var b bytes.Buffer
	compressor := zlib.NewWriter(&b)

	if _, err = compressor.Write([]byte(blobString)); err != nil {
		return nil, err
	}

	compressor.Close()

	err = os.WriteFile(objFile, b.Bytes(), 0700)
	if err != nil {
		return nil, fmt.Errorf("could not write compressed contents of %v to %v", op, objFile)
	}

	return newBlob(id), nil
}

func writeTree(op objectPath) (*Tree, error) {
	_, err := os.Stat(op)
	if err != nil {
		return nil, err
	}

	var content string
	files, err := os.ReadDir(op)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		filePath := filepath.Join(op, file.Name())
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

	if err = os.MkdirAll(objDir, 0700); err != nil {
		return nil, err
	}

	file, err := os.Create(objFile)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	var b bytes.Buffer
	compressor := zlib.NewWriter(&b)

	if _, err = compressor.Write([]byte(treeString)); err != nil {
		return nil, err
	}

	compressor.Close()

	err = os.WriteFile(objFile, b.Bytes(), 0700)
	if err != nil {
		return nil, err
	}

	return newTree(id), nil
}

func formatHexId(obj string, t objectType) (id, objString string, err error) {
	size, content := len(obj), obj
	if t == BLOB {
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

	objString = fmt.Sprintf("%v %d\n%v", t, size, content)
	hasher := sha1.New()

	if _, err = hasher.Write([]byte(objString)); err != nil {
		return "", "", err
	}

	id = hex.EncodeToString(hasher.Sum(nil))

	return id, objString, nil
}
