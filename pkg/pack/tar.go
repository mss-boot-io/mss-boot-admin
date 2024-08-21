/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/10/27 16:33:29
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/10/27 16:33:29
 */

package pack

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Tar 文件压缩为tar
func Tar(root string, src []string, writer io.Writer, ignore ...string) error {
	// gzip write
	gw := gzip.NewWriter(writer)
	defer gw.Close()
	// tar write
	tw := tar.NewWriter(gw)
	defer tw.Close()
	switch len(src) {
	case 0:
		src = []string{"."}
	}
	for i := range src {
		err := filepath.Walk(filepath.Join(root, src[i]), func(path string, info os.FileInfo, err error) error {
			if strings.Contains(path, "node_modules") {
				return nil
			}
			for j := range ignore {
				if strings.Index(path, ignore[j]) > -1 {
					return nil
				}
			}
			if info == nil || info.IsDir() {
				return nil
			}
			var link string
			if info.Mode()&os.ModeSymlink == os.ModeSymlink {
				if link, err = os.Readlink(path); err != nil {
					log.Printf("os.Readlink error: %v\n", err)
					return err
				}
			}
			h, err := tar.FileInfoHeader(info, link)
			if err != nil {
				log.Printf("tar.FileInfoHeader error: %v\n", err)
				return err
			}
			//h.Name = info.Name()
			h.Size = info.Size()
			h.Name = strings.ReplaceAll(strings.ReplaceAll(path, root, "")[1:], "\\", "/")
			h.Mode = int64(info.Mode())
			h.ModTime = info.ModTime()
			//h.Name = strings.ReplaceAll(path, root, "")[1:]

			// 写信息头
			err = tw.WriteHeader(h)
			if err != nil {
				log.Printf("tw.WriteHeader error: %v\n", err)
				return err
			}
			if !info.IsDir() {
				fr, err := os.Open(path)
				if err != nil {
					log.Printf("os.Open error: %v\n", err)
					return err
				}
				defer fr.Close()
				_, err = io.Copy(tw, fr)
				if err != nil {
					log.Printf("io.Copy error: %v\n", err)
					return err
				}
			}
			return nil
		})
		if err != nil {
			log.Printf("filepath.Walk error: %v\n", err)
			return err
		}
	}
	return nil
}

// TarX 解压缩tar文件
func TarX(src, dst string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gr.Close()
	return TarXFromReader(gr, dst)
}

func TarXByContent(content []byte, dst string) error {
	gr, err := gzip.NewReader(bytes.NewBuffer(content))
	if err != nil {
		return err
	}
	defer gr.Close()
	return TarXFromReader(gr, dst)
}

func TarXFromReader(gr io.Reader, dst string) error {
	tr := tar.NewReader(gr)
	_ = os.MkdirAll(dst, os.ModePerm)
	for {
		f, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		filePath := filepath.Join(dst, f.Name)
		log.Printf("tar x file: %s\n", filePath)

		if !strings.HasPrefix(filePath, filepath.Clean(dst)+string(os.PathSeparator)) {
			log.Printf("invalid file path: %s\n", filePath)
			return nil
		}
		if f.FileInfo().IsDir() {
			log.Printf("creating directory: %s\n", filePath)
			_ = os.MkdirAll(filePath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			log.Printf("create file: %s failed\n", filePath)
			return err
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.FileInfo().Mode())
		if err != nil {
			log.Printf("open dst file: %s failed\n", filePath)
			return err
		}

		if _, err = io.Copy(dstFile, tr); err != nil {
			log.Printf("copy tar to dst file error, %s\n", err.Error())
			return err
		}

		dstFile.Close()
	}
	return nil
}
