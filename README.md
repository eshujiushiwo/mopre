# mongo-mopre 工具使用介绍
===


## 功能
 MongoDB point-in-time 恢复工具

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


## startcount&stopcount详解
		在MongoDB的oplog.rs中，ts是如下按如下方式存储的 ：
		"ts" : Timestamp(1419992913, 4)
		在不输入startcount 和stopcount的时候，我们的查询范围是(前后均包含)：
		Timestamp(startts, 0)  ------> Timestamp(stopts, 0)
		在指定了startcount和stopcount的时候，我们的查询范围是(前后均包含):
		Timestamp(startts, startcount)  ------> Timestamp(stopts, stopcount)




##实例





## 问题
		1.性能并未测试
		2.代码结构较乱 
