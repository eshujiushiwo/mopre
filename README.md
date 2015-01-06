# mongo-mopre 工具使用介绍
===


## 功能
 MongoDB point-in-time 恢复工具

## 参数
--fromhost   源地址＜/br＞
--tohost     目标地址＜/br＞
--fromport   源端口（default 27017）＜/br＞
--toport     目标端口（default 27017）＜/br＞
--userName   用户名（如果有）＜/br＞ 
--passWord   密码（如果有）＜/br＞
--database   数据库名＜/br＞
--collection 表名＜/br＞ 
--startts    开始时间戳（unixtimestamp 包含）＜/br＞ 
--stopts     结束时间戳（unixtimestamp 不包含）＜/br＞ 
--logpath    输出日志路径＜/br＞ 
--sliece     是否输出到窗口＜/br＞ 


## 范围迁移使用
1.性能并未测试＜/br＞ 
2.代码结构较乱＜/br＞ 
