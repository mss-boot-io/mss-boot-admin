package pkg

import (
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template/parse"

	"github.com/zealic/xignore"
)

var TemplateIgnore = ".templateignore"
var TemplateParseIgnore = ".templateparseignore"

// getParseKeys get parse keys from template text
func getParseKeys(nodes *parse.ListNode) []string {
	keys := make([]string, 0)
	if nodes == nil {
		return keys
	}
	for a := range nodes.Nodes {
		if actionNode, ok := nodes.Nodes[a].(*parse.ActionNode); ok {
			if actionNode == nil || actionNode.Pipe == nil {
				continue
			}
			for b := range actionNode.Pipe.Cmds {
				if strings.Index(actionNode.Pipe.Cmds[b].String(), ".") == 0 {
					keys = append(keys, actionNode.Pipe.Cmds[b].String()[1:])
				}
			}
		}
	}
	return keys
}

// GetParseFromTemplate get parse keys from template
func GetParseFromTemplate(dir, subPath string) (map[string]string, error) {
	keys := make(map[string]string, 0)
	ignoreDirs := make([]string, 0)
	ignoreFiles := make([]string, 0)
	var allPath bool
	var baseDir string
AGAIN:
	templateResultIgnore, err := xignore.DirMatches(dir,
		&xignore.MatchesOptions{
			Ignorefile: TemplateIgnore,
			Nested:     true, // Handle nested ignorefile
		})
	if err != nil && err != os.ErrNotExist {
		log.Println(err)
		return nil, err
	}
	templateParseResultIgnore, err := xignore.DirMatches(dir,
		&xignore.MatchesOptions{
			Ignorefile: TemplateParseIgnore,
			Nested:     true,
		})
	if err != nil && err != os.ErrNotExist {
		log.Println(err)
		return nil, err
	}
	if templateResultIgnore != nil {
		ignoreDirs = append(ignoreDirs, templateResultIgnore.MatchedDirs...)
		ignoreFiles = append(ignoreFiles, templateResultIgnore.MatchedFiles...)
	}
	if templateParseResultIgnore != nil {
		ignoreDirs = append(ignoreDirs, templateParseResultIgnore.MatchedDirs...)
		ignoreFiles = append(ignoreFiles, templateParseResultIgnore.MatchedFiles...)
	}
	if !allPath {
		baseDir = dir
		dir = filepath.Join(dir, subPath)
		allPath = true
		goto AGAIN
	}
	dir = baseDir
	err = filepath.WalkDir(filepath.Join(dir, subPath), parseTraverse(dir, subPath, keys, ignoreDirs, ignoreFiles))
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return keys, nil
}

func parseTraverse(dir, subPath string, keys map[string]string, ignoreDirs, ignoreFiles []string) fs.WalkDirFunc {
	if keys == nil {
		keys = make(map[string]string)
	}
	return func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			for i := range ignoreDirs {
				if strings.Index(path, filepath.Join(dir, ignoreDirs[i])) == 0 ||
					strings.Index(path, filepath.Join(dir, subPath, ignoreDirs[i])) == 0 {
					return nil
				}
			}
		} else {
			for i := range ignoreFiles {
				if filepath.Join(dir, ignoreFiles[i]) == path ||
					filepath.Join(dir, subPath, ignoreFiles[i]) == path {
					return nil
				}
			}
		}
		{
			tree, err := parse.Parse("path", path, "{{", "}}")
			if err != nil {
				log.Println(err)
				return err
			}
			if tree == nil {
				return nil
			}
			for _, key := range getParseKeys(tree["path"].Root) {
				keys[key] = ""
			}
		}
		if d.IsDir() {
			return nil
		}
		rb, err := ioutil.ReadFile(path)
		if err != nil {
			log.Println(err)
			return err
		}
		{
			tree, err := parse.Parse("file", string(rb), "{{", "}}")
			if err != nil {
				log.Printf("path(%s)'s context error\n", path)
				log.Println(err)
				return err
			}
			if tree == nil {
				return nil
			}
			for _, key := range getParseKeys(tree["file"].Root) {
				keys[key] = ""
			}
		}
		return nil
	}
}
