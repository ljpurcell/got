package got

import "path/filepath"

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

func getRepoPath() filePath {
	return Repo
}

func getIndexPath() filePath {
	return filepath.Join(Repo, IndexFile)
}

func getHeadPath() filePath {
	return filepath.Join(Repo, HeadFile)
}

func getConfigPath() filePath {
	return filepath.Join(Repo, ConfigFile)
}

func getRefsDirPath() filePath {
	return filepath.Join(Repo, RefsDir)
}

func getRefHeadsDirPath() filePath {
	return filepath.Join(Repo, RefsDir, RefHeadsDir)
}

func getRefHeadsMainFilePath() filePath {
	return filepath.Join(Repo, RefsDir, RefHeadsDir, RefHeadsMainFile)
}

func getObjectsDirPath() filePath {
	return filepath.Join(Repo, ObjectsDir)
}
