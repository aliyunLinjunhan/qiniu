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
	mongosAddr       = flag.String("mongos", "localhost:2800", "mongos addr")
	db               = flag.String("d", "test", "db name")
	coll             = flag.String("c", "c", "coll name")
	from             = flag.String("from", "max", "from shard name, use max or shardName")
	stopAtTime       = flag.String("stopAt", "6", "stop time")
	oplogSizeGB      = flag.Int("oplogSize", 40, "oplogSize GB")
	skipMaxSizeCheck = flag.Bool("skipMaxSizeCheck", false, "skip maxSize check")
	skipMidSizeCheck = flag.Bool("skipMidSizeCheck", false, "skip middle size check")
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
	chunks, err := sharding.LoadChunksFromConfigServer(*mongosAddr, ns)
	if err != nil {
		log.Panicln(err)
	}
	log.Info("load shards from config server")
	shards, err := sharding.LoadShards(*mongosAddr)
	if err != nil {
		log.Panicln(err)
	}
	cs := chunks.SplitByShard()

	stopTimeSlice, isVauld := common.IsValidWithStopTime(stopAtTime)
	if !isVauld {
		log.Error("Invalid arguments")
		return
	}
	for {
		if common.IsStopTime(time.Now(), stopTimeSlice) {
			break
		}
		stats, err := sharding.GetShardStats(*mongosAddr, *db, *coll, true)
		if err != nil {
			log.Warn(err)
			time.Sleep(10 * time.Minute)
			continue
		}
		ss := stats.SortBySize()
		if len(ss) < 2 {
			log.Info("only 1 shard, cannot moveChunk")
			return
		}
		fromShard := *from
		if fromShard == "max" {
			fromShard = ss[len(ss)-1].Name
		}
		if len(cs[fromShard]) == 0 {
			log.Info("no chunk to move, exit")
			break
		}
		if !*skipMaxSizeCheck {
			// from 的 shard 存量大小如果小于当前设置的 maxSize 则停止迁移
			if stats.Shards[fromShard].Size+int64(*oplogSizeGB)*1024*1024*1024 <=
				shards[fromShard].MaxSizeMB*1024*1024 {
				if *from == "max" {
					log.Info("max shard size less than maxSize, retry after 10 Minute")
					time.Sleep(10 * time.Minute)
					continue
				}
				log.Infof("shard %v less than maxSize %v, done",
					fromShard, shards[fromShard].MaxSizeMB)
				break
			}
		}
		var to *sharding.ShardStat
		// 选择当前size最小，并且不超过 maxSize 的 shard 进行迁移
		for _, shard := range ss {
			if shard.Name == fromShard {
				continue
			}
			maxSize := shards[shard.Name].MaxSizeMB * 1024 * 1024
			if maxSize == 0 || stats.Shards[shard.Name].Size+
				int64(*oplogSizeGB)*1024*1024*1024 < maxSize {
				to = shard
				break
			}
		}
		if to == nil {
			log.Info("cannot find shard to move to, exit")
			break
		}
		if !*skipMidSizeCheck {
			// 如果源的大小比中位数的size还小，则延迟10分钟再重试
			midSize := ss[len(ss)/2-1].Size
			if stats.Shards[fromShard].Size <= midSize {
				log.Infof("shard %v less than middle shard size %v, retry after 10 Minute",
					fromShard, midSize)
				time.Sleep(10 * time.Minute)
				continue
			}
			// 如果最大的shard和最小的shard大小差距小于1G，则延迟10分钟后再重试
			if stats.Shards[fromShard].Size-to.Size < 1024*1024*1024 {
				log.Info("max shard size - min shard size < 1G, retry after 10 Minute")
				time.Sleep(10 * time.Minute)
				continue
			}
		}
		// 随机迁移一个chunk
		selectChunkID := rand.Intn(len(cs[fromShard]))
		chunk := cs[fromShard][selectChunkID]
		cs.Remove(fromShard, selectChunkID)
		err = sharding.MoveChunk(s, ns, fromShard, to.Name, chunk)
		if err != nil {
			log.Warn(err)
			if strings.Contains(err.Error(), "Cannot move chunk: the maximum number of documents for a chunk is") {
				// Cannot move chunk: the maximum number of documents for a chunk is 570206, the maximum chunk size is 134217728, average document size is 306. Found 689346 documents in chunk  ns: rs13.rs2
				// 由于chunk信息是程序启动的时候拉取的，未来可能会大到一定程度而分裂
				// 这种迁移失败可以先忽略
				continue
			}
			if strings.Contains(err.Error(), "Could not acquire collection lock") {
				// Could not acquire collection lock for rs13.rs2 to migrate chunks, due to timed out waiting for
				// 这种错误延迟一段时间重试可以恢复
				time.Sleep(time.Minute)
				continue
			}
			if strings.Contains(err.Error(), "this shard is currently donating chunk") {
				// Unable to start new migration because this shard is currently donating chunk
				// 出现这种报错说明shard正在执行迁移操作，延迟一段时间再重试
				time.Sleep(time.Minute)
				continue
			}
			// 非预期的错误延迟30分钟后再次重试
			log.Info("========= unexpected error, retry after 20 Minute ==========")
			time.Sleep(20 * time.Minute)
		}
	}
}

func main() {
	flag.Parse()
	loopMoveChunk()
}
