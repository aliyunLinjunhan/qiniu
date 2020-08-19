package main

import (
	"github.com/qiniu/xlog.v1"
	"qiniu.com/kodo/lib/cc/config"
	"qiniu.com/kodo/lib/mgo3"
	"qiniu.com/kodo/rs/rs/v2/qtbl"
)

const (
	BUCKET_NOMIGRATE = 0
	BUCKET_MIGRATING = 1
	BUCKET_MIGRATED  = 2
)

type Conf struct {
	RoMgo           interface{}        `json:"ro_mgo"`
	RwConf          RwMgoConf          `json:"rw_mgo"`
	MigrateBuckConf MigrateBuckMgoConf `json:"migrate_buck_mgo"`
	NeedMovedBuck   []uint32           `json:"need_moved_buck"`
	BatchNum        int                `json:"batch_num"`
	ListBatchNum    int                `json:"list_batch_num"`
	ListInterval    int                `json:"list_interval"`
}

type MigrateBuck struct {
	Itbl       uint32 `bson:"itbl"`
	BuckStatus int    `bson:"buck_status"`
	Mark       string `bson:"mark"`
	MigrateNum int    `bson:"migrate_num"`
}

type RwMgoConf struct {
	mgo3.Config
}

type MigrateBuckMgoConf struct {
	mgo3.Config
}

func main() {

	var conf Conf
	config.Init("f", "tool", "migrate_buck.conf")

	err := config.Load(&conf)
	if err != nil {
		return
	}

	MigrateBucket(conf)
}

func MigrateBucket(conf Conf) {

	tool, err := NewMigrateTool(conf)
	if err != nil {
		xlog.Error(err.Error())
		return
	}
	defer tool.DestoryMigrateToolMgo()

	stg, _, err := qtbl.OpenReadonlyStorage(conf.RoMgo)
	if err != nil {
		panic(err)
	}
	tool.stg = stg
	xl := xlog.NewDummy()

	// 根据 itbl 获取bucket信息
	var bucketMessage *MigrateBuck
	for _, v := range conf.NeedMovedBuck {
		isExit, entry, err := tool.ExitMigrateBuck(v)
		if err != nil {
			xl.Error(err.Error())
			return
		}
		if isExit && entry.BuckStatus == BUCKET_MIGRATED {
			xl.Info("", v, "had been migrated")
			continue
		}

		if isExit {
			bucketMessage = entry
		} else {
			bucketMessage = &MigrateBuck{Itbl: v, BuckStatus: BUCKET_NOMIGRATE}
			err := tool.InsertMigrateBuck(bucketMessage)
			if err != nil {
				xl.Error("", v, "has been migrated")
				return
			}
		}

		err = tool.migrateBuck(xl, bucketMessage)
		if err != nil {
			xl.Error(err.Error())
		}
	}
}
