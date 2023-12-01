package pkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// GetInstallPath Different systems get different installation paths
func GetInstallPath() string {
	var path string
	if IsWindows() {
		path = `C:\Program Files\nps`
	} else {
		path = "/etc/nps"
	}
	return path
}

// GetAppPath Get the absolute path to the running directory
func GetAppPath() string {
	if path, err := filepath.Abs(filepath.Dir(os.Args[0])); err == nil {
		return path
	}
	return os.Args[0]
}

// IsWindows Determine whether the current system is a Windows system?
func IsWindows() bool {
	if runtime.GOOS == "windows" {
		return true
	}
	return false
}

// GetTmpPath interface pid file path
func GetTmpPath() string {
	var path string
	if IsWindows() {
		path = GetAppPath()
	} else {
		path = "/tmp"
	}
	return path
}

type release struct {
	TagName string `json:"tag_name"`
}

// GetLatestVersion get generate-tool version
func GetLatestVersion() string {
	// get version
	data, err := http.Get("https://api.github.com/repos/mss-boot-io/micro-service-gen-tool/releases/latest")
	if err != nil {
		log.Fatal(err.Error())
	}
	b, err := ioutil.ReadAll(data.Body)
	if err != nil {
		log.Fatal(err)
	}
	rl := new(release)
	_ = json.Unmarshal(b, &rl)
	return rl.TagName
}

func copyStaticFile(srcPath, bin string) string {
	defer os.RemoveAll(srcPath)
	binPath, _ := filepath.Abs(os.Args[0])
	if !IsWindows() {
		if _, err := copyFile(filepath.Join(srcPath, bin), "/usr/bin/"+bin); err != nil {
			if _, err := copyFile(filepath.Join(srcPath, bin), "/usr/local/bin/"+bin); err != nil {
				log.Fatalln(err)
			} else {
				copyFile(filepath.Join(srcPath, bin), "/usr/local/bin/"+bin+"-update")
				chMod("/usr/local/bin/"+bin+"-update", 0755)
				binPath = "/usr/local/bin/" + bin
			}
		} else {
			copyFile(filepath.Join(srcPath, bin), "/usr/bin/"+bin+"-update")
			chMod("/usr/bin/"+bin+"-update", 0755)
			binPath = "/usr/bin/" + bin
		}
	} else {
		copyFile(filepath.Join(srcPath, bin+".exe"), filepath.Join(GetAppPath(), bin+"-update.exe"))
		copyFile(filepath.Join(srcPath, bin+".exe"), filepath.Join(GetAppPath(), bin+".exe"))
	}
	chMod(binPath, 0755)
	return binPath
}

func CopyDir(srcPath string, destPath string) error {
	//检测目录正确性
	if srcInfo, err := os.Stat(srcPath); err != nil {
		fmt.Println(err.Error())
		return err
	} else {
		if !srcInfo.IsDir() {
			e := errors.New("SrcPath is not the right directory!")
			return e
		}
	}
	if destInfo, err := os.Stat(destPath); err != nil {
		return err
	} else {
		if !destInfo.IsDir() {
			e := errors.New("DestInfo is not the right directory!")
			return e
		}
	}
	err := filepath.Walk(srcPath, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if !f.IsDir() {
			destNewPath := strings.Replace(path, srcPath, destPath, -1)
			log.Println("copy file ::" + path + " to " + destNewPath)
			copyFile(path, destNewPath)
			if !IsWindows() {
				chMod(destNewPath, 0766)
			}
		}
		return nil
	})
	return err
}

//生成目录并拷贝文件
func copyFile(src, dest string) (w int64, err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return
	}
	defer srcFile.Close()
	//分割path目录
	destSplitPathDirs := strings.Split(dest, string(filepath.Separator))

	//检测时候存在目录
	destSplitPath := ""
	for index, dir := range destSplitPathDirs {
		if index < len(destSplitPathDirs)-1 {
			destSplitPath = destSplitPath + dir + string(filepath.Separator)
			b, _ := pathExists(destSplitPath)
			if b == false {
				log.Println("mkdir:" + destSplitPath)
				//创建目录
				err := os.Mkdir(destSplitPath, os.ModePerm)
				if err != nil {
					log.Fatalln(err)
				}
			}
		}
	}
	dstFile, err := os.Create(dest)
	if err != nil {
		return
	}
	defer dstFile.Close()

	return io.Copy(dstFile, srcFile)
}

//检测文件夹路径时候存在
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func chMod(name string, mode os.FileMode) {
	if !IsWindows() {
		os.Chmod(name, mode)
	}
}
