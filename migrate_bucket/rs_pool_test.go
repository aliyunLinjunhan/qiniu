package main

import (
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/golib/assert"
	"strconv"
	"testing"

	"github.com/qiniu/xlog.v1"
	"qiniu.com/kodo/lib/mgo3"
)

func TestRsPool(t *testing.T) {

	newRwSession := mgo3.Open(&mgo3.Config{
		Host: "localhost:27017",
		DB:   "test_new_rs_db",
		Coll: "test_rs",
	})
	defer newRwSession.Close()
	migrateBuckSess := mgo3.Open(&mgo3.Config{
		Host: "localhost:27017",
		DB:   "test_migrate_buck_db",
		Coll: "test_buck",
	})
	defer migrateBuckSess.Close()

	newRwSession.Coll.DropCollection()
	migrateBuckSess.Coll.DropCollection()

	xl := xlog.NewDummy()
	pool := NewRsPool(30, 1, newRwSession, migrateBuckSess, xl)
	bucketMessage := &MigrateBuck{Itbl: 1, BuckStatus: BUCKET_NOMIGRATE}
	err := migrateBuckSess.Coll.Insert(bucketMessage)
	assert.NoError(t, err)
	err = migrateBuckSess.Coll.EnsureIndex(mgo.Index{
		Key:    []string{"itbl"},
		Unique: true,
	})
	assert.NoError(t, err)

	// 第一批
	for i := 0; i < 90; i++ {
		count := strconv.Itoa(i)
		m := bson.M{
			"_id":      "aqzsin:" + count,
			"mimeType": "image/jpeg",
			"fsize":    111111,
		}
		err := pool.InsertRsData(m)
		assert.NoError(t, err)
	}
	err = pool.ClearRsData()
	assert.NoError(t, err)

	count, err := pool.newRwSession.Coll.Count()
	assert.NoError(t, err)
	assert.Equal(t, 90, count)

	var buck MigrateBuck
	err = pool.migrateBuckSess.Coll.Find(bson.M{"itbl": 1}).One(&buck)
	assert.NoError(t, err)
	assert.Equal(t, 90, buck.MigrateNum)
	assert.Equal(t, BUCKET_MIGRATED, buck.BuckStatus)
	assert.Equal(t, "", buck.Mark)

	// 第二批
	for i := 100; i < 200; i++ {
		count := strconv.Itoa(i)
		m := bson.M{
			"_id":      "aqzsin:" + count,
			"mimeType": "image/jpeg",
			"fsize":    111111,
		}
		err := pool.InsertRsData(m)
		assert.NoError(t, err)
	}
	err = pool.ClearRsData()
	assert.NoError(t, err)

	count, err = pool.newRwSession.Coll.Count()
	assert.NoError(t, err)
	assert.Equal(t, 190, count)

	err = pool.migrateBuckSess.Coll.Find(bson.M{"itbl": 1}).One(&buck)
	assert.NoError(t, err)
	assert.Equal(t, 190, buck.MigrateNum)
	assert.Equal(t, BUCKET_MIGRATED, buck.BuckStatus)
	assert.Equal(t, "", buck.Mark)

	// 第三批
	for i := 200; i < 300; i++ {
		count := strconv.Itoa(i)
		m := bson.M{
			"_id":      "aqzsin:" + count,
			"mimeType": "image/jpeg",
			"fsize":    111111,
		}
		err := pool.InsertRsData(m)
		assert.NoError(t, err)
	}

	count, err = pool.newRwSession.Coll.Count()
	assert.NoError(t, err)
	assert.Equal(t, 280, count)

	err = pool.migrateBuckSess.Coll.Find(bson.M{"itbl": 1}).One(&buck)
	assert.NoError(t, err)
	assert.Equal(t, 280, buck.MigrateNum)
	assert.Equal(t, BUCKET_MIGRATING, buck.BuckStatus)
	assert.Equal(t, "289", buck.Mark)

	err = pool.ClearRsData()
	assert.NoError(t, err)

	count, err = pool.newRwSession.Coll.Count()
	assert.NoError(t, err)
	assert.Equal(t, 290, count)

	err = pool.migrateBuckSess.Coll.Find(bson.M{"itbl": 1}).One(&buck)
	assert.NoError(t, err)
	assert.Equal(t, 290, buck.MigrateNum)
	assert.Equal(t, BUCKET_MIGRATED, buck.BuckStatus)
	assert.Equal(t, "", buck.Mark)
}
