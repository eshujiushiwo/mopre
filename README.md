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
		--startts    开始时间戳（unixtimestamp 包含） 
		--stopts     结束时间戳（unixtimestamp 不包含） 
		--logpath    输出日志路径
		--sliece     是否输出到窗口 


## 问题
		1.性能并未测试
		2.代码结构较乱 
