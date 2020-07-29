# movechunk

## 设计目标

mongodb sharding默认是按chunk个数进行均衡，
对于range分片，各个chunk大小可能差别很大。
如果仍然按chunk个数均衡可能会导致各个shard数据量不均衡，
会出现某些shard磁盘占用空间满了但是有些shard磁盘还很空。

movechunk 迁移的目标是保证各个shard数据量均衡而不是chunk个数均衡。

## 实现原理

movechunk 启动后会获取各个 shard 的当前存储的数据量大小，从大到小排序，
从数据量最大的shard往数据量最小的shard迁移（当前是单个并发，未来可以考虑多个shard并发迁移）。


## 实现细节

movechunk 通过 crontab 每天凌晨1点启动，可以通过 stopAt 参数设置停止时间，避开完高峰。

默认从存储量最大的 shard 开始迁出，可以通过 from 参数指定从某个 shard 迁出。
迁出过程中如果迁出的 shard 大小比中位数的shard大小还小则停止迁出。可以通过 skipMidSizeCheck 这个参数来禁用。

每个 shard 可以指定 maxSize，mongodb sharding 自己的 balancer 在 shard 达到 maxSize 之后就不再继续迁入，movechunk 也保留了此功能，并且在迁出的时候如果shard小于 maxSize 会停止迁出。停止迁出功能可以通过 skipMaxSizeCheck 来禁用。

由于 mongodb 计算 maxSize 包含 local 数据库，这里需要指定 oplog 大小。

## 用法示例

```
qboxserver@zz84:~/zhoujinjun$ crontab -l | grep movechunk
0 1 * * * /home/qboxserver/zhoujinjun/movechunk -c rs2 -d rs13 -from max -oplogSize 40 -stopAt 18 -skipMaxSizeCheck >> /home/qboxserver/zhoujinjun/movechunk.log 2>&1
qboxserver@zz84:~/zhoujinjun$
```
