package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/go-irodsclient/irods/util"

	log "github.com/sirupsen/logrus"
)

func main() {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "main",
	})

	// Parse cli parameters
	flag.Parse()
	args := flag.Args()

	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "Give a local source path and an iRODS destination path!\n")
		os.Exit(1)
	}

	srcPath := args[0]
	destPath := args[1]

	// Read account configuration from YAML file
	yaml, err := ioutil.ReadFile("account.yml")
	if err != nil {
		logger.Errorf("err - %v", err)
		panic(err)
	}

	account, err := types.CreateIRODSAccountFromYAML(yaml)
	if err != nil {
		logger.Errorf("err - %v", err)
		panic(err)
	}

	logger.Debugf("Account : %v", account.MaskSensitiveData())

	// Create a file system
	appName := "upload"
	filesystem, err := fs.NewFileSystemWithDefault(account, appName)
	if err != nil {
		logger.Errorf("err - %v", err)
		panic(err)
	}

	defer filesystem.Release()

	// convert src path into absolute path
	srcPath, err = filepath.Abs(srcPath)
	if err != nil {
		logger.Errorf("err - %v", err)
		panic(err)
	}

	err = filesystem.UploadFile(srcPath, destPath, "", false, nil)
	if err != nil {
		logger.Errorf("err - %v", err)
		panic(err)
	}

	fsentry, err := filesystem.Stat(destPath)
	if err != nil {
		logger.Errorf("err - %v", err)
		panic(err)
	}

	if fsentry.Type == fs.FileEntry {
		fmt.Printf("Successfully uploaded a file %s to %s, size = %d\n", srcPath, destPath, fsentry.Size)
	} else {
		// dir
		srcFileName := util.GetIRODSPathFileName(srcPath)
		destFilePath := util.MakeIRODSPath(destPath, srcFileName)

		fsentry2, err := filesystem.Stat(destFilePath)
		if err != nil {
			logger.Errorf("err - %v", err)
			panic(err)
		}

		if fsentry2.Type == fs.FileEntry {
			fmt.Printf("Successfully uploaded a file %s to %s, size = %d\n", srcPath, destFilePath, fsentry2.Size)
		} else {
			logger.Errorf("Unkonwn file type - %s", fsentry2.Type)
		}
	}
}
