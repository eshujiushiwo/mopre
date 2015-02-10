# mongo-mopre 工具使用介绍
===


## 功能
 MongoDB point-in-time 恢复工具

##2015.02.10更新：
		新增参数all，默认为no,当设置为yes的时候，将无视所指定的db和ns，恢复所有的符合时间段的oplog数据。
		即，如果为yes，则进行所有db数据的恢复。

## 参数
		--fromhost   源地址
		--tohost     目标地址
		--fromport   源端口（default 27017）
		--toport     目标端口（default 27017）
		--userName   用户名（如果有）
		--passWord   密码（如果有）
		--database   数据库名
		--collection 表名 
		--startts    开始时间戳（unixtimestamp 详细解释见下文） 
		--stopts     结束时间戳（unixtimestamp 详细解释见下文） 
		--startcount 从开始时间戳的第几个操作开始（default 0 详细解释见下文）
		--stopcount  在结束时间戳的第几个操作结束（default 0 详细解释见下文）
		--logpath    输出日志路径
		--sliece     是否输出到窗口 
		--cpu		 来源为mongos的时候，并发使用的cpu核数
		--all		 是否恢复所有数据库（default no）


## startcount&stopcount详解
		在MongoDB的oplog.rs中，ts是如下按如下方式存储的 ：
		"ts" : Timestamp(1419992913, 4)
		在不输入startcount 和stopcount的时候，我们的查询范围是(前后均包含)：
		Timestamp(startts, 0)  ------> Timestamp(stopts, 0)
		在指定了startcount和stopcount的时候，我们的查询范围是(前后均包含):
		Timestamp(startts, startcount)  ------> Timestamp(stopts, stopcount)




##测试实例

		背景：拥有10点的快照(X)，需要恢复数据库 (DB) 到12点某个误操作之前
		假设：开始时间戳,startts为a,startcount为0,误操作之前最后的时间戳,stopts为b，stopcount为n
		主机A（原复制集中一个从节点，出问题后从以非复制集方式启动，保护现场）
		主机B（零时用于数据恢复的机器）先使用快照X恢复到10点的状态

		./mopre --fromhost A --tohost B --fromport 27017 --toport 27017  --startts a --startcount 0 --stopts b --stopcount n  --database DB  --logpath /tmp/test.log --slience yes

		如果恢复所有数据库，而非指定DB，则（此时指定的database DB 不生效）：
		./mopre --fromhost A --tohost B --fromport 27017 --toport 27017  --startts a --startcount 0 --stopts b --stopcount n  --database DB  --logpath /tmp/test.log --slience yes --all yes











## 问题
		1.性能并未测试
		2.代码结构较乱 
