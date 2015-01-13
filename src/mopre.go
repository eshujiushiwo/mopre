package main

import (
	"flag"
	"fmt"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"io"
	"log"
	"os"
	"runtime" //for goroutine
	//"reflect" //for test
	"regexp" //all database
	"strings"
)

var logfile *os.File
var logger *log.Logger

type MongoInfo struct {
	fromhost   string
	tohost     string
	userName   string
	passWord   string
	database   string
	collection string
	startts    int64
	stopts     int64
	startcount int64
	stopcount  int64
	fromport   string
	toport     string
	ismongos   bool
	srcShards  map[string]string
	//srcShards    []string
	srcClient    *mgo.Session
	destClient   *mgo.Session
	srcDBConn    *mgo.Database
	srcCollConn  *mgo.Collection
	destDBConn   *mgo.Database
	destCollConn *mgo.Collection
}

func GetMongoDBUrl(addr, userName, passWord string, port string) string {
	var mongoDBUrl string

	if port == "no" {
		if userName == "" || passWord == "" {

			mongoDBUrl = "mongodb://" + addr

		} else {

			mongoDBUrl = "mongodb://" + userName + ":" + passWord + "@" + addr

		}

	} else {

		if userName == "" || passWord == "" {

			mongoDBUrl = "mongodb://" + addr + ":" + port

		} else {

			mongoDBUrl = "mongodb://" + userName + ":" + passWord + "@" + addr + ":" + port

		}

	}
	return mongoDBUrl
}

//Get the MongoInfo
func Newmongoinfo(fromhost, tohost, userName, passWord, database, collection string, startts, stopts, startcount, stopcount int64, fromport, toport string) *MongoInfo {

	mongoinfo := &MongoInfo{fromhost, tohost, userName, passWord, database, collection, startts, stopts, startcount, stopcount, fromport, toport, false, nil, nil, nil, nil, nil, nil, nil}
	mongoinfo.srcShards = make(map[string]string)
	logger.Println(len(mongoinfo.srcShards))
	return mongoinfo
}

//get the src type (repl set or shard cluster)
func (mongoinfo *MongoInfo) Getsrctype() {
	command := bson.M{"isMaster": 1}
	result := bson.M{}
	mongoinfo.srcClient.Run(command, &result)
	if result["msg"] == "isdbgrid" {
		mongoinfo.ismongos = true
		logger.Println("src is mongos")
		mongoinfo.Restoreforshard()

	} else {
		logger.Println("src is not mongos,may be mongod.")

	}

}

func (mongoinfo *MongoInfo) Restoreforshard() {
	var result bson.M
	shards := mongoinfo.srcClient.DB("config").C("shards").Find(nil).Iter()
	defer shards.Close()
	var srchost, srcid string
	var ok bool
	for shards.Next(&result) {

		if srchost, ok = result["host"].(string); ok {

			if strings.Contains(srchost, "/") {

				if srcid, ok = result["_id"].(string); ok {
					logger.Println(srchost)
					mongoinfo.srcShards[srcid] = mongoinfo.Getthenodeofshard(srchost)

					//mongoinfo.srcShards = append(mongoinfo.srcShards, mongoinfo.Getthenodeofshard(srchost))

				}

			} else {
				continue
			}
		}
	}

}
func (mongoinfo *MongoInfo) Getthenodeofshard(srchost string) string {
	var err error
	var selectHost string
	host1 := strings.Split(srchost, "/")[1]
	host2 := strings.Split(host1, ",")[1]

	mongourl := GetMongoDBUrl(host2, mongoinfo.userName, mongoinfo.passWord, "no")
	logger.Println(mongourl)
	mongoclient, err := mgo.Dial(mongourl)
	if err != nil {
		logger.Println(err)
	}
	defer mongoclient.Close()

	var replConf, replStatus bson.M
	replConfMap := make(map[interface{}]bson.M)
	replStatusMap := make(map[interface{}]interface{})
	command := bson.M{"replSetGetStatus": 1}

	mongoclient.DB("admin").Run(command, &replStatus)
	mongoclient.DB("local").C("system.replset").Find(nil).One(&replConf)

	if confMembers, isConfMembersLegal := replConf["members"].([]interface{}); isConfMembersLegal {
		for _, confMember := range confMembers {
			if bsonConfMember, isBsonConfMember := confMember.(bson.M); isBsonConfMember {
				hostAndDelay := bson.M{"host": bsonConfMember["host"], "slaveDelay": bsonConfMember["slaveDelay"]}
				replConfMap[bsonConfMember["_id"]] = hostAndDelay
			}
		}
	}

	if statusMembers, isStatusMembersLegal := replStatus["members"].([]interface{}); isStatusMembersLegal {
		for _, statusMember := range statusMembers {
			if bsonStatusMember, isBsonStatusMember := statusMember.(bson.M); isBsonStatusMember {
				replStatusMap[bsonStatusMember["_id"]] = bsonStatusMember["state"]
			}
		}
	}

	logger.Println("replStatus:", replStatusMap)

	logger.Println("replConf:", replConfMap)

	for id, state := range replStatusMap {

		if state == 1 || state == 2 {
			if replConfMap[id]["slaveDelay"] == 0 || replConfMap[id]["slaveDelay"] == nil {
				if host, ok := replConfMap[id]["host"].(string); ok {
					selectHost = host
				}
			}
			if state == 2 {
				break
			}
		}
	}

	logger.Println("oplog sync node selected:", selectHost)

	return selectHost

}

//Apply Oplog to restore
func (mongoinfo *MongoInfo) ApplyOplog(oplog bson.M, coll string) {
	op := oplog["op"]
	dbcoll := strings.SplitN(coll, ".", 2)

	switch op {

	case "i":

		err_i := mongoinfo.destClient.DB(dbcoll[0]).C(dbcoll[1]).Insert(oplog["o"])
		if err_i != nil {
			logger.Println("Error:insert failed:", err_i)
			os.Exit(1)

		}

	case "u":

		err_u := mongoinfo.destClient.DB(dbcoll[0]).C(dbcoll[1]).Update(oplog["o2"], oplog["o"])
		if err_u != nil {
			logger.Println("Error:update failed:", err_u)
			os.Exit(1)
		}

	case "d":

		err_d := mongoinfo.destClient.DB(dbcoll[0]).C(dbcoll[1]).Remove(oplog["o"])
		if err_d != nil {
			logger.Println("Error:delete failed:", err_d)
			os.Exit(1)
		}

	}
}

//Start restore
//including create the client ,get the mongotimestamp and compare the ns

func (mongoinfo *MongoInfo) Conn() {
	var err, err1 error
	//set the client
	srcMongoUri := GetMongoDBUrl(mongoinfo.fromhost, mongoinfo.userName, mongoinfo.passWord, mongoinfo.fromport)
	destMongoUri := GetMongoDBUrl(mongoinfo.tohost, mongoinfo.userName, mongoinfo.passWord, mongoinfo.toport)

	mongoinfo.srcClient, err = mgo.Dial(srcMongoUri)

	logger.Println("The source url is " + srcMongoUri)
	if err != nil {
		logger.Println("connect to", mongoinfo.fromhost, ":", mongoinfo.fromport, "failed")

	}
	logger.Println("connect to", mongoinfo.fromhost, ":", mongoinfo.fromport, "successed")

	mongoinfo.destClient, err1 = mgo.Dial(destMongoUri)

	if err1 != nil {
		logger.Println("connect to", mongoinfo.tohost, ":", mongoinfo.toport, "failed")
	}
	logger.Println("connect to", mongoinfo.tohost, ":", mongoinfo.toport, "successed")

}

func (mongoinfo *MongoInfo) StartRestore(addr string, ch chan int) {
	var oplogDB *mgo.Collection
	if addr == "repl" {
		oplogDB = mongoinfo.srcClient.DB("local").C("oplog.rs")
	} else {
		mongourl := GetMongoDBUrl(addr, mongoinfo.userName, mongoinfo.passWord, "no")
		mongoclient, _ := mgo.Dial(mongourl)
		oplogDB = mongoclient.DB("local").C("oplog.rs")
	}
	var result bson.M

	//unixtimestamp to mongotimstamp
	var tmp1, tmp2 int64
	tmp1 = mongoinfo.startts<<32 + mongoinfo.startcount
	tmp2 = mongoinfo.stopts<<32 + mongoinfo.stopcount
	var mongostartts, mongostopts bson.MongoTimestamp
	mongostartts = bson.MongoTimestamp(tmp1)
	mongostopts = bson.MongoTimestamp(tmp2)
	oplogquery := bson.M{"ts": bson.M{"$gte": mongostartts, "$lte": mongostopts}}

	oplogIter := oplogDB.Find(oplogquery).LogReplay().Sort("$natural").Iter()

	for oplogIter.Next(&result) {
		//if we use the item collection && compare the ns and collection we selected
		if mongoinfo.collection != "" && result["ns"] != mongoinfo.database+"."+mongoinfo.collection {
			continue
		}

		//all datbase

		if mongoinfo.collection == "" {

			match, _ := regexp.MatchString(mongoinfo.database+".*", result["ns"].(string))

			if match == false {
				continue

			}

		}

		timestamp := result["ts"].(bson.MongoTimestamp) >> 32

		ct := result["ts"].(bson.MongoTimestamp) - bson.MongoTimestamp(timestamp<<32)
		if addr == "repl" {
			logger.Println("from:", mongoinfo.fromhost, "MongoTimestamp:", result["ts"], "; UnixTimestamp:", timestamp, ";Count pos:", ct)
		} else {
			logger.Println("from:", addr, "MongoTimestamp:", result["ts"], "; UnixTimestamp:", timestamp, ";Count pos:", ct)
		}
		mongoinfo.ApplyOplog(result, result["ns"].(string))
	}
	ch <- 1

}

func main() {
	var fromhost, tohost, userName, passWord, database, collection string
	var startts, stopts, startcount, stopcount int64
	var cpu int
	var fromport, toport, logpath, slience string
	var err1 error
	var multi_logfile []io.Writer
	flag.StringVar(&fromhost, "fromhost", "", "the source host")
	flag.StringVar(&tohost, "tohost", "", "the dest host")
	flag.StringVar(&userName, "userName", "", "the username")
	flag.StringVar(&passWord, "passWord", "", "the password")
	flag.StringVar(&database, "database", "", "the database")
	flag.StringVar(&collection, "collection", "", "the collection")
	flag.Int64Var(&startts, "startts", 0, "the time to start")
	flag.Int64Var(&stopts, "stopts", 0, "the time to stop")
	flag.StringVar(&fromport, "fromport", "27017", "the src port")
	flag.StringVar(&toport, "toport", "27017", "the dest port")
	flag.StringVar(&logpath, "logpath", "", "the log path ")
	flag.StringVar(&slience, "slience", "no", "slient or not")
	flag.Int64Var(&startcount, "startcount", 0, "the op start count of startts")
	flag.Int64Var(&stopcount, "stopcount", 0, "the op stop count of stopts")
	flag.IntVar(&cpu, "cpu", 1, "the cpu nums ")

	flag.Parse()

	if fromhost != "" && tohost != "" && database != "" && logpath != "" && startts != 0 && stopts != 0 {

		logfile, err1 = os.OpenFile(logpath, os.O_RDWR|os.O_CREATE, 0666)
		defer logfile.Close()
		if err1 != nil {
			logger.Println(err1.Error())
			os.Exit(-1)
		}

		if slience == "yes" {

			multi_logfile = []io.Writer{
				logfile,
			}
		} else {
			multi_logfile = []io.Writer{
				logfile,
				os.Stdout,
			}
		}

		logfiles := io.MultiWriter(multi_logfile...)
		logger = log.New(logfiles, "\r\n", log.Ldate|log.Ltime|log.Lshortfile)

		logger.Println("=====job start.=====")
		logger.Println("start init colletion")
		mongoinfo := Newmongoinfo(fromhost, tohost, userName, passWord, database, collection, startts, stopts, startcount, stopcount, fromport, toport)
		mongoinfo.Conn()
		mongoinfo.Getsrctype()
		logger.Println("the max process num is set to :", cpu)
		if len(mongoinfo.srcShards) != 0 {
			chs := make([]chan int, len(mongoinfo.srcShards))

			i := 0
			for _, fhost := range mongoinfo.srcShards {

				chs[i] = make(chan int)

				runtime.GOMAXPROCS(cpu)
				go mongoinfo.StartRestore(fhost, chs[i])
				logger.Println(fhost)
				i++
				for _, cha := range chs {
					<-cha

				}
			}

		} else {
			ch := make(chan int, len(mongoinfo.srcShards))
			go mongoinfo.StartRestore("repl", ch)
			<-ch

		}

		defer mongoinfo.srcClient.Close()
		defer mongoinfo.destClient.Close()

		logger.Println("=====Done.=====")
		os.Exit(0)

	} else {

		fmt.Println("Please use -help to check the usage")
		fmt.Println("At least need fromhost,tohost,database,logpath,startts and stopts")

	}

}
