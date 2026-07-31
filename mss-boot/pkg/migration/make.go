package migration

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/12 09:15:17
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/12 09:15:17
 */

import (
	"bytes"
	"embed"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"text/template"
	"time"

	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

//go:embed *.tpl
var FS embed.FS

var Migrate = &Migration{
	version: make(map[int]func(db *gorm.DB, version string) error),
}

type Migration struct {
	db      *gorm.DB
	version map[int]func(db *gorm.DB, version string) error
	mutex   sync.Mutex
	Model   Version
}

func (e *Migration) GetDb() *gorm.DB {
	return e.db
}

func (e *Migration) SetDb(db *gorm.DB) {
	e.db = db
}

func (e *Migration) SetModel(v Version) {
	e.Model = v
}

func (e *Migration) SetVersion(k int, f func(db *gorm.DB, version string) error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if e.version == nil {
		e.version = make(map[int]func(db *gorm.DB, version string) error)
	}
	e.version[k] = f
}

func (e *Migration) cloneModel() Version {
	return reflect.New(reflect.TypeOf(e.Model).Elem()).Interface().(Version)
}

func (e *Migration) CreateVersion(tx *gorm.DB, v string) error {
	m := reflect.New(reflect.TypeOf(e.Model).Elem()).Interface().(Version)
	m.SetVersion(v)
	return tx.Create(m).Error
}

func (e *Migration) Migrate() {
	if e.version == nil {
		e.version = make(map[int]func(db *gorm.DB, version string) error)
	}
	versions := make([]int, 0)
	for k := range e.version {
		versions = append(versions, k)
	}
	if !sort.IntsAreSorted(versions) {
		sort.Ints(versions)
	}
	for _, v := range versions {
		m := e.cloneModel()
		m.SetVersion(fmt.Sprintf("%d", v))
		exist, err := m.Done(e.db)
		if err != nil {
			slog.Error("get migration version", slog.Any("error", err))
			os.Exit(-1)
		}
		if exist {
			continue
		}
		err = (e.version[v])(e.db, strconv.Itoa(v))
		if err != nil {
			log.Fatalf("migrate version %d error: %v", v, err)
		}
	}
}

func GetFilename(s string) int {
	s = filepath.Base(s)
	return cast.ToInt(s[:13])
}

func GenFile(system bool, path string) error {
	t1, err := template.ParseFS(FS, "migrate.tpl")
	if err != nil {
		slog.Error("parse template error", slog.Any("err", err))
		return err
	}
	m := map[string]string{}
	m["GenerateTime"] = strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
	m["Package"] = "custom"
	if system {
		m["Package"] = "system"
	}
	var b1 bytes.Buffer
	err = t1.Execute(&b1, m)
	if err != nil {
		slog.Error("execute template error", slog.Any("err", err))
		return err
	}
	if system {
		fileCreate(b1, filepath.Join(path, "system", m["GenerateTime"]+"_migrate.go"))
	} else {
		fileCreate(b1, filepath.Join(path, "custom", m["GenerateTime"]+"_migrate.go"))
	}
	return nil
}

func fileCreate(content bytes.Buffer, name string) {
	if !pkg.PathExist(filepath.Dir(name)) {
		err := pkg.PathCreate(filepath.Dir(name))
		if err != nil {
			log.Fatalln(err)
		}
	}
	file, err := os.Create(name)
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatalln(err)
		}
	}(file)
	if err != nil {
		log.Println(err)
	}
	_, err = file.WriteString(content.String())
	if err != nil {
		log.Println(err)
	}
}
