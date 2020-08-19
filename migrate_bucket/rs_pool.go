package main

import (
	"qiniu.com/kodo/rs/buckfiledel"
	"strings"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"

	"github.com/qiniu/xlog.v1"
	"qiniu.com/kodo/lib/mgo3"
)

// 这是一个缓存需要迁移的元数据池子，用来捆绑成批数据插入数据库, 并在插入后进行持久化
type RsPool struct {
	Max             int
	Datas           []interface{}
	NextEnterPoint  int // 指向下一次数据进入到Datas的位置
	EndDbPoint      int // 指向最后一个已经被插入到数据库的位置
	newRwSession    *mgo3.Session
	migrateBuckSess *mgo3.Session
	Itbl            uint32
	xl              *xlog.Logger
}

func NewRsPool(max int, Itbl uint32, newRwSession, migrateBuckSess *mgo3.Session, xl *xlog.Logger) *RsPool {
	if max <= 0 {
		panic("max cannot be less than 0")
	}
	if newRwSession == nil {
		panic("newRwSession cannot be nil")
	}
	if migrateBuckSess == nil {
		panic("migrateBuckSess cannot be nil")
	}
	return &RsPool{
		Max:             max,
		Datas:           make([]interface{}, max),
		NextEnterPoint:  0,
		EndDbPoint:      max - 1,
		newRwSession:    newRwSession,
		migrateBuckSess: migrateBuckSess,
		Itbl:            Itbl,
		xl:              xl,
	}
}

// 往RsPool中插入数据
func (rp *RsPool) InsertRsData(data bson.M) (err error) {

	rp.Datas[rp.NextEnterPoint] = data
	if rp.NextEnterPoint == rp.EndDbPoint {
		mark := strings.Split(data["_id"].(string), ":")[1]
		buckfiledel.Retry(rp.xl, func() (shouldRetry bool) {
			err = rp.newRwSession.Coll.Insert(rp.Datas...)
			if err != nil {
				if !mgo.IsDup(err) {
					rp.xl.Error("insert into rw failed !", err.Error())
					return true
				}
			}
			return false
		}, 200*time.Millisecond, 5*time.Second, 3)
		if err != nil {
			if !mgo.IsDup(err) {
				return err
			}
		}

		err = rp.migrateBuckSess.Coll.Update(bson.M{"itbl": rp.Itbl}, bson.M{"$set": bson.M{"mark": mark, "buck_status": BUCKET_MIGRATING}})
		if err != nil {
			rp.xl.Error("update migrate buck failed !", err.Error())
			return err
		}
		err = rp.migrateBuckSess.Coll.Update(bson.M{"itbl": rp.Itbl}, bson.M{"$inc": bson.M{"migrate_num": rp.Max}})
		if err != nil {
			rp.xl.Error("update migrate buck failed !", err.Error())
			return err
		}
	}
	rp.NextEnterPoint = (rp.NextEnterPoint + 1) % rp.Max
	return nil
}

// 将RsPool中剩余未插入到数据库的数据清入数据库
func (rp *RsPool) ClearRsData() error {

	left := (rp.EndDbPoint + 1) % rp.Max
	right := rp.NextEnterPoint
	length := 0
	if left == rp.NextEnterPoint {
		// 数据池是空的，跳过

	} else if left < right {
		length = right - left
		copyData := make([]interface{}, length)

		copy(copyData, rp.Datas[left:right])
		err := rp.newRwSession.Coll.Insert(copyData...)
		if err != nil {
			if !mgo.IsDup(err) {
				rp.xl.Error("insert into rw failed !", err.Error())
				return err
			}
		}
	} else if left > right {
		copy1 := make([]interface{}, rp.Max-left)
		copy2 := make([]interface{}, right)
		length = rp.Max - left + right
		copy(copy1, rp.Datas[left:])
		copy(copy2, rp.Datas[:right])

		copyData := append(copy1, copy2...)
		err := rp.newRwSession.Coll.Insert(copyData...)
		if err != nil {
			if !mgo.IsDup(err) {
				rp.xl.Error("insert into rw failed !", err.Error())
				return err
			}
		}
	}
	err := rp.migrateBuckSess.Coll.Update(bson.M{"itbl": rp.Itbl}, bson.M{"$set": bson.M{"mark": "", "buck_status": BUCKET_MIGRATED}})
	if err != nil {
		xlog.Error("update migrate buck failed !", err.Error())
		return err
	}
	err = rp.migrateBuckSess.Coll.Update(bson.M{"itbl": rp.Itbl}, bson.M{"$inc": bson.M{"migrate_num": length}})
	if err != nil {
		xlog.Error("update migrate buck failed !", err.Error())
		return err
	}
	rp.EndDbPoint = (rp.NextEnterPoint + rp.Max - 1) % rp.Max
	return nil
}
