package pkg

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PathCreate create path
func PathCreate(dir string) error {
	return os.MkdirAll(dir, os.ModePerm)
}

// PathExist path exist
func PathExist(addr string) bool {
	s, err := os.Stat(addr)
	if err != nil {
		slog.Error("PathExist")
		log.Println(err)
		return false
	}
	return s.IsDir()
}

func FileOpen(content bytes.Buffer, name string, mode os.FileMode) error {
	file, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		log.Println(err)
		return err
	}
	defer file.Close()

	changeStr := strings.ReplaceAll(content.String(), `\$`, `$`)
	changeStr = strings.ReplaceAll(changeStr, `\}`, "}")
	changeStr = strings.ReplaceAll(changeStr, `\{`, "{")
	_, err = file.WriteString(changeStr)
	if err != nil {
		log.Println(err)
	}
	return err

}

// FileCreate create file
func FileCreate(content bytes.Buffer, name string) error {
	return FileOpen(content, name, 0666)
}

type ReplaceHelper struct {
	// Root path
	Root string
	// OldText need to replace text
	OldText string
	// NewText new text
	NewText string
}

func (h *ReplaceHelper) DoWork() error {
	return filepath.Walk(h.Root, h.walkCallback)
}

func (h *ReplaceHelper) walkCallback(path string, f os.FileInfo, err error) error {

	if err != nil {
		return err
	}
	if f == nil {
		return nil
	}
	if f.IsDir() {
		log.Println("DIR:", path)
		return nil
	}

	buf, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(buf)
	log.Printf("h.OldText: %s \n", h.OldText)
	log.Printf("h.NewText: %s \n", h.NewText)

	//替换
	newContent := strings.Replace(content, h.OldText, h.NewText, -1)

	//重新写入
	err = os.WriteFile(path, []byte(newContent), 0)
	if err != nil {
		return err
	}

	return err
}

func FileMonitoringById(ctx context.Context, filePth string, id string, group string, hook func(context.Context, string, string, []byte)) {
	f, err := os.Open(filePth)
	if err != nil {
		log.Fatalln(err)
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Fatalln(err)
		}
	}(f)

	rd := bufio.NewReader(f)
	_, err = f.Seek(0, 2)
	if err != nil {
		return
	}
	for {
		if ctx.Err() != nil {
			break
		}
		line, err := rd.ReadBytes('\n')
		// 如果是文件末尾不返回
		if err == io.EOF {
			time.Sleep(500 * time.Millisecond)
			continue
		} else if err != nil {
			log.Fatalln(err)
		}
		go hook(ctx, id, group, line)
	}
}

// GetFileSize 获取文件大小
func GetFileSize(filename string) int64 {
	var result int64
	err := filepath.Walk(filename, func(path string, f os.FileInfo, err error) error {
		result = f.Size()
		return nil
	})
	if err != nil {
		log.Println(err)
		return 0
	}
	return result
}

// GetCurrentPath 获取当前路径，比如：E:/abc/data/test
func GetCurrentPath() string {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}

// FileCopy copy file
func FileCopy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, sourceFileStat.Mode())
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

// GetSubPath get directory's subject path
func GetSubPath(directory string) ([]string, error) {
	dirs, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	subPath := make([]string, 0)
	for i := range dirs {
		if dirs[i].IsDir() {
			subPath = append(subPath, dirs[i].Name())
		}
	}
	return subPath, nil
}

func substr(s string, pos, length int) string {
	runes := []rune(s)
	l := pos + length
	if l > len(runes) {
		l = len(runes)
	}
	return string(runes[pos:l])
}
