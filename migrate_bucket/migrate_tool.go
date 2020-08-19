package main

import (
	"encoding/json"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"

	"github.com/qiniu/xlog.v1"
	"qiniu.com/kodo/lib/mgo3"
	"qiniu.com/kodo/rs/buckfiledel"
	"qiniu.com/kodo/rs/rs/v2/qtbl/proto"
)

type MigrateTool struct {
	newRwSession    *mgo3.Session
	migrateBuckSess *mgo3.Session
	stg             proto.ReadonlyStorage
	batchNum        int
	listBatchNum    int
	listInterval    int
}

func NewMigrateTool(conf Conf) (*MigrateTool, error) {

	migrateBuckSess := mgo3.Open(&conf.MigrateBuckConf.Config)
	err := migrateBuckSess.Coll.EnsureIndex(mgo.Index{
		Key:    []string{"itbl"},
		Unique: true,
	})
	if err != nil {
		xlog.Error("set index failed, ", err.Error())
		return nil, err
	}

	return &MigrateTool{
		newRwSession:    mgo3.Open(&conf.RwConf.Config),
		migrateBuckSess: migrateBuckSess,
		batchNum:        conf.BatchNum,
		listBatchNum:    conf.ListBatchNum,
		listInterval:    conf.ListInterval,
	}, nil
}

func (mtg *MigrateTool) DestoryMigrateToolMgo() {
	mtg.migrateBuckSess.Close()
	mtg.newRwSession.Close()
}

func (mtg *MigrateTool) ExitMigrateBuck(itbl uint32) (bool, *MigrateBuck, error) {
	// 判断该 bucket	是不是之前迁移过，如果迁移过返回true，并将之前记录的信息返回；否贼返回false
	var entry MigrateBuck
	err := mtg.migrateBuckSess.Coll.Find(bson.M{"itbl": itbl}).One(&entry)
	if err != nil {
		if err == mgo.ErrNotFound {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, &entry, nil
}

func (mtg *MigrateTool) InsertMigrateBuck(mb *MigrateBuck) error {
	return mtg.migrateBuckSess.Coll.Insert(mb)
}

func (mtg *MigrateTool) migrateBuck(xlog *xlog.Logger, migrateBuck *MigrateBuck) error {

	iter, err := mtg.stg.List(&proto.ListArgs{Itbl: migrateBuck.Itbl}, migrateBuck.Mark, 300, xlog)
	if err != nil {
		xlog.Error("ListAllRoMessage list error")
		return err
	}
	var doc *proto.ListResult
	var num int
	buckfiledel.Retry(xlog, func() (shouldRetry bool) {
		doc, err = iter.Next()
		if err != nil {
			if err != proto.ErrNotFound {
				return true
			}
		}
		return false
	}, 200*time.Millisecond, 5*time.Second, 3)
	if err != nil {
		if err == proto.ErrNotFound {
			err = mtg.migrateBuckSess.Coll.Update(bson.M{"itbl": migrateBuck.Itbl}, bson.M{"$set": bson.M{"mark": "", "is_migrate": BUCKET_MIGRATED}})
			if err != nil {
				xlog.Error("update migrate buck failed !", err.Error())
				return err
			}
			return nil
		} else {
			return err
		}
	}
	num++

	// 初始化rsPool
	rsPool := NewRsPool(mtg.batchNum, migrateBuck.Itbl, mtg.newRwSession, mtg.migrateBuckSess, xlog)

	for {
		if doc == nil {
			break
		}
		// 1. 获取信息全量rs信息
		get, err := mtg.stg.Get(&proto.EntryKey{Key: doc.Key, Itbl: migrateBuck.Itbl}, nil, xlog)
		if err != nil {
			xlog.Error("get ", doc.Key, "failed", err)
			return err
		}
		json1, _ := json.Marshal(get)
		xlog.Info(string(json1), "is being moved")

		// 2. 将rs信息保存下来
		allMess := bson.M{}
		keys := get.ListKeys()
		for _, k := range keys {
			allMess[k] = get.Get(k)
		}
		// 对取出来的数据做必要的胡处理
		handleRsMap(&allMess)

		err = rsPool.InsertRsData(allMess)
		if err != nil {
			xlog.Error("insert into rspool err", err)
			return err
		}

		if num == mtg.listBatchNum {
			time.Sleep(time.Duration(mtg.listInterval) * time.Millisecond)
			num = 0
		}
		buckfiledel.Retry(xlog, func() (shouldRetry bool) {
			doc, err = iter.Next()
			if err != nil {
				if err != proto.ErrNotFound {
					return true
				}
			}
			return false
		}, 200*time.Millisecond, 5*time.Second, 3)
		if err != nil && err != proto.ErrNotFound {
			return err
		}
		num++
	}
	err = rsPool.ClearRsData()
	if err != nil {
		xlog.Error("clear rs data err", err)
		return err
	}

	return nil
}

func handleRsMap(m *bson.M) {
	(*m)["osize"] = (*m)["fsize"]
}

//func (mtg *MigrateTool) getBucketMessage(itbl uint32, xl *xlog.Logger) (*MigrateBuck, error) {
//
//	entry, err := tblmgr.NewNullAuth(rpc.DefaultClient).GetByItbl(nil, mtg.buckService, itbl)
//	if err != nil {
//		xl.Error(err.Error())
//		return nil, err
//	}
//	return &MigrateBuck{Uid: entry.Uid, Tbl: entry.Tbl, Itbl: entry.Itbl, BuckStatus: BUCKET_NOMIGRATE, Mark: "", MigrateNum: 0}, nil
//}
