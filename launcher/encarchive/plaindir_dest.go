package encarchive

import (
	"github.com/HouzuoGuo/laitos/misc"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var (
	// PlainDirDestParentDir is the directory under which extract destinations will be created.
	PlainDirDestParentDir = os.TempDir()

	// PlainDirDestPrefix is the prefix name of temporary directory which is to hold decrypted program data.
	PlainDirDestPrefix = "plaindir-laitos-extracted-bundle"
)

// MakePlainDestDir creates a new empty directory for holding extracted program data and returns the directory's path.
func MakePlainDestDir() (string, error) {
	dest, err := ioutil.TempDir(PlainDirDestParentDir, PlainDirDestPrefix)
	if err != nil {
		return "", err
	}
	misc.DefaultLogger.Warning("MakePlainDestDir", dest, nil, "successfully created temporary directory for decrypted program data")
	return dest, err
}

// DestroyPlainDestDir recursively deletes the directory and returns true only upon successful operation.
func DestoryPlainDestDir(fullPath string) bool {
	err := os.RemoveAll(fullPath)
	if err == nil {
		misc.DefaultLogger.Warning("DestroyPlainDestDir", fullPath, nil, "successfully destroyed temporary directory")
	} else {
		misc.DefaultLogger.Warning("DestroyPlainDestDir", fullPath, err, "failed to erase the directory")
	}
	return err == nil
}

/*
TryDestroyAllPlainDestDirs deletes all plain directory destinations that hold decrypted program data left by previous
laitos launches.
*/
func TryDestroyAllPlainDestDirs() {
	dirs, err := ioutil.ReadDir(PlainDirDestParentDir)
	if err != nil {
		return
	}
	for _, info := range dirs {
		if strings.HasPrefix(info.Name(), PlainDirDestPrefix) {
			DestoryPlainDestDir(filepath.Join(PlainDirDestParentDir, info.Name()))
		}
	}
}
