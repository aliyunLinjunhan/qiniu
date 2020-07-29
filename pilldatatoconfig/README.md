### pilldatatoconfig 使用说明

1. 明确需要灌入chunk的sharding对应mongos、shardsvr的host信息。
2. 明确需要进行灌入chunk的集合（包括数据库和集合信息），并开启sharding模式。

    这里以为pilldata.pilldata添加chunk为例：
    1. 设置分片服务器：
    ```
   db.runCommand({ addshard:"127.0.0.1:27100" })
   db.runCommand({ addshard:"127.0.0.1:27101" })
   db.runCommand({ addshard:"127.0.0.1:27017" })
   或者
   sh.addShard("127.0.0.1:27100")
    ```
    2. 设置要分片的数据库
    ```
    db.runCommand({ enablesharding:"pilldata" })
    或者：
    sh.enableSharding("pilldata")
    ```
   3. 通过命令`sh.status()`查看sharding情况，如果前面俩步设置成功应该会出现一下内容
    ```
    {  "_id" : "pilldata",  "primary" : "shard1",  "partitioned" : true }
    ```
   4. 为集合创建索引
   ```
    db.pilldata.ensureIndex({"userId":1})
    ```
   5. 设置要分片的集合
   ```
   db.runCommand({ shardcollection: "pilldata.pilldata", key: { "userId":1}})
   或者
   sh.shardCollection("pilldata.pilldata",{"usrId":1})
   ```
   6. 关闭balancing
   ```
   sh.disableBalancing("pilldata.pilldata")
    ```
   7. 再次通过命令`sh.status()`查看sharding情况，如果前面步骤成功会有一下内容
   ```
    {  "_id" : "pilldata",  "primary" : "shard1",  "partitioned" : true }
                    pilldata.pilldata
                            shard key: { "userId" : 1 }
                            unique: false
                            balancing: false
                            chunks:
                                    shard1	1
                            { "userId" : { "$minKey" : 1 } } -->> { "userId" : { "$maxKey" : 1 } } on : shard1 Timestamp(1, 0)
    ```
  
3. 开始使用pilldatatoconfig工具

    1. 现在需要在mongos中手动对coll进行一次分裂，这是因为在程序中无法进行分裂，不知道是什么原因。。。。。。
    ```
    sh.splitAt("db.coll", { key: splitpoint}) 
    其中db是数据库，coll是集合，key是分裂的键值，splitpoint是分裂点 splitpoint = 10*需要插入的chunks总数
    ```
    2. pilldatatoconfig 启动参数
        -configAddr   config服务端host 默认是localhost:15350
        
       	-db           需要灌入chunks的db 默认是pilldata_db
       	
       	-coll         需要灌入chunks的文档 默认是pilldata_coll
       	
       	-pillNum      需要灌入chunks的数量 默认是10000
       	
       	-key          决定chunks分配的键值 默认是userId
       	
       	-shards       要分配到的shard 默认是shard1,shard2
        
        -mode         chunks灌入的方式 1是交替灌入 2是间隔灌入 默认是1
    


###### ====注意点====： 务必进行一次手动分裂；只能向大重复灌入比如先灌入了10000，后面想灌入1000000是可以的，但是反过来则不行，重新灌入也要进行手动分裂

###### 参考文档:https://qwendy.github.io/2016/12/22/MongoDB-Sharding常用命令/