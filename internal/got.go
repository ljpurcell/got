package got

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	BLOB   objectType = "blob"
	TREE   objectType = "tree"
	COMMIT objectType = "commit"
)

type (
	filePath   = string
	id         = string
	objectPath = string
	objectType = string
)

type user struct {
	Name  string
	Email string
}

type Config struct {
	User user
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
	// TODO: Start here
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

	objectDb := getObjectsDirPath()
	objDir := filepath.Join(objectDb, treeId[:2])
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
	headFile := getHeadPath()
	headRef, err := os.ReadFile(headFile)
	if err != nil {
		return err
	}

	ref := strings.Split(string(headRef), ":")
	pathBits := strings.Split(strings.TrimSpace(ref[1]), "/")

	repo := getRepoPath()
	pathToRef := append([]string{repo}, pathBits...)
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

	objectDb := getObjectsDirPath()
	objDir := filepath.Join(objectDb, id[:2])
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

	fmt.Printf("Created commit %s", id)

	return cb.commit, nil
}

func newCommitBuilder() *commitBuilder {
	return &commitBuilder{
		commit: &Commit{},
	}
}

func GetConfig() (Config, error) {
	configFile, err := os.Open(getConfigPath())
	if err != nil {
		return Config{}, fmt.Errorf("could not open config file: %w", err)
	}

	c := Config{User: user{}}

	scanner := bufio.NewScanner(configFile)
	for scanner.Scan() {
		line := strings.ToLower(strings.TrimSpace(scanner.Text()))

		if strings.HasPrefix(line, "name") {
			v, err := getValueFromConfigLine(line)
			if err != nil {
				return Config{}, fmt.Errorf("could not parse name in config file: %w", err)
			}
			c.User.Name = v
		}

		if strings.HasPrefix(line, "email") {
			v, err := getValueFromConfigLine(line)
			if err != nil {
				return Config{}, fmt.Errorf("could not parse email in config file: %w", err)
			}
			c.User.Email = v
		}

	}

	return c, nil
}

func getValueFromConfigLine(line string) (string, error) {
	bits := strings.Split(line, "=")
	if len(bits) != 2 {
		return "", errors.New("incorrect format")
	}
	return strings.TrimSpace(bits[1]), nil
}

func newBlob(id id) *Blob {
	return &Blob{object{Id: id, Type: BLOB}}
}

func newTree(id id) *Tree {
	return &Tree{object{Id: id, Type: TREE}}
}

func Init() error {
	repoPath := getRepoPath()
	if _, err := os.Stat(repoPath); err == nil {
		return fmt.Errorf("%s already exists", repoPath)
	}

	configPath := getConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("%s already exists", configPath)
	}

	rw := fs.FileMode(0777)

	if err := os.WriteFile(getConfigPath(), []byte("ref: refs/heads/main"), rw); err != nil {
		return fmt.Errorf("could not write to HEAD file: %w", err)
	}

	if err := os.MkdirAll(getRefsDirPath(), rw); err != nil {
		return fmt.Errorf("could not create refs directory path: %w", err)
	}

	index, err := os.Create(getIndexPath())
	if err != nil {
		return fmt.Errorf("could not create index file: %w", err)
	}
	defer index.Close()

	return nil
}

func GetObjectFile(id id) (*os.File, error) {
	objectsPath := getObjectsDirPath()
	objects, err := os.ReadDir(objectsPath)
	if err != nil {
		return nil, err
	}

	for _, dir := range objects {
		if dir.Name() == id[:2] {
			dPath := filepath.Join(objectsPath, dir.Name())
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

func getIdFromRef(ref filePath) (id, error) {
	path := filepath.Join(getRepoPath(), ref)

	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("could not get ref %q: %w", path, err)
	}

	if id(b) == "" {
		return "", errors.New("no head commit")
	}

	return id(b), nil
}

// TODO: unfinished
func getHeadCommit() (*Commit, error) {
	b, err := os.ReadFile(getHeadPath())
	if err != nil {
		return nil, err
	}

	ref := string(b)

	var head id

	if strings.HasPrefix(ref, "ref") {
		refPath := strings.Split(ref, ": ")[1]
		head, err = getIdFromRef(refPath)
		if err != nil {
			return nil, err
		}
	}

	// Get Object file using ID - GetObjectFile
	commitFile, err := GetObjectFile(head)
	if err != nil {
		return nil, err
	}
	defer commitFile.Close()

	// Get tree ID from commit
	scanner := bufio.NewScanner(commitFile)

	var tree id
	line := 1
	for scanner.Scan() {
		if line == 1 && !strings.HasPrefix(scanner.Text(), "commit") {
			return nil, fmt.Errorf("file %s does not contain commit object", head)
		}

		if line == 2 {
			if !strings.HasPrefix(scanner.Text(), "tree") {
				return nil, fmt.Errorf("commit %v incorrectly formatted", head)
			}

			tree = strings.Split(scanner.Text(), " ")[1]
			break
		}

		line++
	}

	treeFile, err := GetObjectFile(tree)
	if err != nil {
		return nil, err
	}
	defer treeFile.Close()

	contents, err := os.ReadFile(treeFile.Name())

	fmt.Printf("Content of tree file:\n\n%s", contents)

	// Load into commit struct
	return nil, nil
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

	objDir := filepath.Join(getObjectsDirPath(), id[:2])
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

	objDir := filepath.Join(getObjectsDirPath(), id[:2])
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
