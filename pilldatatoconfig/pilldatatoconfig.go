package main

import (
	"flag"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/qiniu/log.v1"
	"os"
	"strconv"
	"strings"
	"time"
)

type M map[string]interface{}

const (
	MODE_ALTERNATE = "1" // 交替
	MODE_INTERVAL = "2"	// 间隔
)

var (
	configAddr   = flag.String("configSvr", "localhost:15350", "config server addr")
	db           = flag.String("db", "pilldata_db", "the db need to be fill the data")
	coll         = flag.String("coll", "pilldata_coll", "the coll need to be fill the data")
	pillNum      = flag.Int("pillNum", 10000, "the nums of chunk needto be pill")
	key          = flag.String("key", "userId", "the key required by splitting chunks")
	shards       = flag.String("shards", "shard1,shard2", "all shards that need to be allocated")
	//lastmodEpoch = flag.String("lastmodEpoch", "", "the lastmodEpoch from Mongo")
	mode = flag.String("mode", MODE_ALTERNATE, "Select the method with chunks inserted")
)

func main() {

	flag.Parse()

	// 日志
	nowTime := time.Now()
	fileName := "./pilldatatoconfig-" + strconv.Itoa(nowTime.Year()) + nowTime.Month().String() + strconv.Itoa(nowTime.Day()) + strconv.Itoa(nowTime.Hour()) + strconv.Itoa(nowTime.Minute()) + ".log"
	logFile, err := os.Create(fileName)
	defer logFile.Close()
	if err != nil {
		log.Fatalln("open log file error !")
	}
	xlog := log.New(logFile, "[Debug]", log.LstdFlags)

	// mongo
	session, err := mgo.Dial(*configAddr)
	if err != nil {
		xlog.Error(err)
	}
	defer session.Close()
	session.SetMode(2, true)
	session.SetSocketTimeout(8 * time.Second)
	session.SetSyncTimeout(8 * time.Second)

	dbConfig := session.DB("config")
	collChunks := dbConfig.C("chunks")
	session.EnsureSafe(&mgo.Safe{WMode: "majority"})

	// shard
	allshards := strings.Split(*shards, ",")
	var length int = len(allshards)
	var now int = 0

	// lastmodEpoch
	var result M
	query := M{"_id": *db + "." + *coll + "-" + *key + "_MinKey"}
	err = collChunks.Find(query).One(&result)
	if err != nil {
		xlog.Error("get lastmodEpoch failed !")
		return
	}
	lastmodEpoch := result["lastmodEpoch"]

	for i := 10; i < 10*(*pillNum); i += 10 {
		if *mode == MODE_ALTERNATE {
			now = (now + 1) % length
		} else if *mode == MODE_INTERVAL {
			if i % 100 == 0 {
				now = (now + 1) % length
			}
		} else {
			xlog.Error("the mode input is error !")
			return
		}

		start := strconv.Itoa(i)
		end := strconv.Itoa(i + 10)
		var doc M = make(map[string]interface{})
		doc["_id"] = *db + "." + *coll + "-" + *key + "_" + start + ".0"
		doc["lastmod"], _ = bson.NewMongoTimestamp(time.Unix(2, 0).Local(), uint32(i/10+1))
		doc["lastmodEpoch"] = lastmodEpoch.(bson.ObjectId)
		doc["ns"] = *db + "." + *coll
		doc["min"] = M{*key: i}
		doc["max"] = M{*key: i + 10}
		doc["shard"] = allshards[now]

		err := collChunks.Insert(doc)
		if err != nil {
			if mgo.IsDup(err) {
				xlog.Warn("insert", doc["_id"], "warn:", err)
			} else {
				xlog.Error("insert", doc["_id"], "err:", err)
			}
		} else {
			xlog.Info("insert", start, "--->", end)
		}
	}

	change := M{"$set": M{"max": M{*key: 10}}}
	err = collChunks.Update(query, change)
	if err != nil {
		xlog.Error("update MinKey failed", err)
	} else {
		xlog.Info("update MinKey successed")
	}

}
