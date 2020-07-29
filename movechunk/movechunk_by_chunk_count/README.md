# movechunk_by_chunk_count

和 movechunk 类似，`movechunk_by_chunk_count` 也是用于chunk迁移，
`movechunk_by_chunk_count` 设计目标是根据各个shard的chunk个数进行迁移，
同时支持只迁移指定前缀的chunk(mongodb自带均衡的不支持)，
从而达到将客户的请求均衡到不同的shard的目地，提升整体服务能力。
但是，如果客户的key是单调递增的是起不到均衡请求的目地，
客户的请求需要在一定程度上是分散的才能用此工具。
