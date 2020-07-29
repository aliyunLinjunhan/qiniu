package main

import (
	"flag"
	"math/rand"
	"strings"
	"time"

	"github.com/globalsign/mgo"
	"github.com/qiniu/log.v1"
	"qiniu.com/kodo/rs2/roll/mongo/sharding"
	"qiniu.com/kodo/tools/mongodb/sharding/movechunk/common"

	_ "github.com/qiniu/version"
	_ "qiniu.com/kodo/lib/autoua"
	_ "qiniu.com/kodo/lib/profile"
)

var (
	mongosAddr = flag.String("mongos", "localhost:2800", "mongos addr")
	db         = flag.String("d", "test", "db name")
	coll       = flag.String("c", "c", "coll name")
	from       = flag.String("from", "max", "from shard name, use max or shardName")
	stopAtTime = flag.String("stopAt", "6", "stop time")
	prefix     = flag.String("prefix", "", "only move chunks with prefix") // 仅支持range分片
)

func loopMoveChunk() {
	log.Info("start")
	ns := *db + "." + *coll
	active, err := sharding.IsBalancerActive(*mongosAddr)
	if err != nil {
		log.Panicln(err)
	}
	if active {
		log.Info("balancer is running, exit")
		return
	}
	log.Info("connect to mongos")
	s, err := mgo.Dial(*mongosAddr)
	if err != nil {
		log.Panicln(err)
	}
	s.SetSocketTimeout(time.Hour)
	s.SetSyncTimeout(time.Hour)
	defer s.Close()
	log.Info("load chunks from config server")
	tmpChunks, err := sharding.LoadChunksFromConfigServer(*mongosAddr, ns)
	if err != nil {
		log.Panicln(err)
	}
	var chunks sharding.Chunks
	for _, chunk := range tmpChunks {
		if strings.HasPrefix(chunk.Min.StrVal, *prefix) {
			chunks = append(chunks, chunk)
		}
	}
	log.Info("load shards from config server")
	shards, err := sharding.LoadShards(*mongosAddr)
	if err != nil {
		log.Panicln(err)
	}
	cs := chunks.SplitByShard()
	for shardName := range shards {
		if _, ok := cs[shardName]; !ok {
			cs[shardName] = nil
		}
	}
	stopTimeSlice, isVauld := common.IsValidWithStopTime(stopAtTime)
	if !isVauld {
		log.Error("Invalid arguments")
		return
	}
	for {
		if common.IsStopTime(time.Now(), stopTimeSlice) {
			break
		}
		scs := cs.SortByChunkCount()
		fromShard := *from
		if fromShard == "max" {
			fromShard = scs[len(scs)-1].Shard
		}
		toShard := scs[0].Shard
		if len(cs[fromShard])-len(cs[toShard]) <= 1 {
			break
		}
		selectChunkID := rand.Intn(len(cs[fromShard]))
		chunk := cs[fromShard][selectChunkID]
		err = sharding.MoveChunk(s, ns, fromShard, toShard, chunk)
		if err != nil {
			log.Warn(err)
			time.Sleep(time.Minute)
			continue
		}
		cs.Move(fromShard, toShard, selectChunkID)
	}
}

func main() {
	flag.Parse()
	loopMoveChunk()
}
