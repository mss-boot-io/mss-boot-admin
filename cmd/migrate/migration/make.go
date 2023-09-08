package migration

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/12 09:15:17
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/12 09:15:17
 */

import (
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	"github.com/spf13/cast"
	"gorm.io/gorm"
)

var Migrate = &Migration{
	version: make(map[int]func(db *gorm.DB, version string) error),
}

type Migration struct {
	db      *gorm.DB
	version map[int]func(db *gorm.DB, version string) error
	mutex   sync.Mutex
}

func (e *Migration) GetDb() *gorm.DB {
	return e.db
}

func (e *Migration) SetDb(db *gorm.DB) {
	e.db = db
}

func (e *Migration) SetVersion(k int, f func(db *gorm.DB, version string) error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.version[k] = f
}

func (e *Migration) Migrate() {
	versions := make([]int, 0)
	for k := range e.version {
		versions = append(versions, k)
	}
	if !sort.IntsAreSorted(versions) {
		sort.Ints(versions)
	}
	var err error
	var count int64
	for _, v := range versions {
		err = e.db.Table("mss_boot_migration").Where("version = ?", v).Count(&count).Error
		if err != nil {
			log.Fatalf("get migration version error: %v", err)
		}
		if count > 0 {
			log.Println(count)
			count = 0
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
