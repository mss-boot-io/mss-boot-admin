/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/10/27 16:40:42
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/10/27 16:40:42
 */

package pack

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Zip 文件压缩zip
func Zip(root string, src []string, writer io.Writer, ignore ...string) error {
	// zip write
	zw := zip.NewWriter(writer)
	defer zw.Close()
	switch len(src) {
	case 0:
		src = []string{"."}
	}
	for i := range src {
		err := filepath.Walk(filepath.Join(root, src[i]), func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			for j := range ignore {
				if strings.Index(path, ignore[j]) > -1 {
					return nil
				}
			}
			h, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			h.Method = zip.Deflate
			h.Name = strings.ReplaceAll(strings.ReplaceAll(path, root, "")[1:], "\\", "/")
			h.Modified = info.ModTime()
			w, err := zw.CreateHeader(h)
			if err != nil {
				return err
			}
			fr, err := os.Open(path)
			if err != nil {
				return err
			}
			defer fr.Close()
			// 写信息头
			_, err = io.Copy(w, fr)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Unzip 解压缩zip文件
func Unzip(src, dst string) error {
	// zip reader
	or, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer or.Close()
	fmt.Sprintf("src: %v, dst: %v", src, dst)

	return unzipFromReader(or.Reader, dst)
}

func UnzipByContent(content []byte, dst string) error {
	// zip reader
	or, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return err
	}

	return unzipFromReader(*or, dst)
}

func unzipFromReader(or zip.Reader, dst string) error {
	for _, f := range or.File {
		filePath := filepath.Join(dst, f.Name)
		fmt.Sprintf("unzipping file: %v", filePath)

		if !strings.HasPrefix(filePath, filepath.Clean(dst)+string(os.PathSeparator)) {
			fmt.Sprintf("invalid file path: %v", filePath)
			return nil
		}
		if f.FileInfo().IsDir() {
			fmt.Sprintf("creating directory: %v", filePath)
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			fmt.Sprintf("create file: %v failed", filePath)
			panic(err)
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			fmt.Sprintf("open dst file: %v failed", filePath)
			panic(err)
		}

		fileInArchive, err := f.Open()
		if err != nil {
			fmt.Sprintf("open src file: %v failed", f.Name)
			panic(err)
		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			fmt.Sprintf("copy file to dst file: %v failed", dstFile)
			panic(err)
		}

		dstFile.Close()
		fileInArchive.Close()
	}
	return nil
}
