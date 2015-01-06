package main

import (
	"flag"
	"fmt"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"io"
	"log"
	"os"
	"strings"
	//"time"
	//"reflect"
	"regexp" //all database
)

var logfile *os.File
var logger *log.Logger

type MongoInfo struct {
	fromhost     string
	tohost       string
	userName     string
	passWord     string
	database     string
	collection   string
	startts      int64
	stopts       int64
	fromport     string
	toport       string
	srcClient    *mgo.Session
	destClient   *mgo.Session
	srcDBConn    *mgo.Database
	srcCollConn  *mgo.Collection
	destDBConn   *mgo.Database
	destCollConn *mgo.Collection
}

func GetMongoDBUrl(addr, userName, passWord string, port string) string {
	var mongoDBUrl string

	if userName == "" || passWord == "" {

		mongoDBUrl = "mongodb://" + addr + ":" + port

	} else {

		mongoDBUrl = "mongodb://" + userName + ":" + passWord + "@" + addr + ":" + port

	}

	return mongoDBUrl
}

//Get the MongoInfo
func Newmongoinfo(fromhost, tohost, userName, passWord, database, collection string, startts, stopts int64, fromport, toport string) *MongoInfo {

	mongoinfo := &MongoInfo{fromhost, tohost, userName, passWord, database, collection, startts, stopts, fromport, toport, nil, nil, nil, nil, nil, nil}

	return mongoinfo
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
		}

	case "u":

		err_u := mongoinfo.destClient.DB(dbcoll[0]).C(dbcoll[1]).Update(oplog["o2"], oplog["o"])
		if err_u != nil {
			logger.Println("Error:update failed:", err_u)
		}

	case "d":

		err_d := mongoinfo.destClient.DB(dbcoll[0]).C(dbcoll[1]).Remove(oplog["o"])
		if err_d != nil {
			logger.Println("Error:delete failed:", err_d)
		}

	}
}

//Start restore
//including create the client ,get the mongotimestamp and compare the ns

func (mongoinfo *MongoInfo) StartRestore() {
	var err, err1 error
	//set the client
	srcMongoUri := GetMongoDBUrl(mongoinfo.fromhost, mongoinfo.userName, mongoinfo.passWord, mongoinfo.fromport)
	destMongoUri := GetMongoDBUrl(mongoinfo.tohost, mongoinfo.userName, mongoinfo.passWord, mongoinfo.toport)

	mongoinfo.srcClient, err = mgo.Dial(srcMongoUri)
	defer mongoinfo.srcClient.Close()

	logger.Println("The source url is " + srcMongoUri)
	if err != nil {
		logger.Println("connect to", mongoinfo.fromhost, "failed")

	}
	logger.Println("connect to", mongoinfo.fromhost, "successed")

	mongoinfo.destClient, err1 = mgo.Dial(destMongoUri)
	defer mongoinfo.destClient.Close()

	if err1 != nil {
		logger.Println("connect to", mongoinfo.tohost, "failed")
	}
	logger.Println("connect to", mongoinfo.tohost, "successed")

	var result bson.M
	oplogDB := mongoinfo.srcClient.DB("local").C("oplog.rs")
	//unixtimestamp to mongotimstamp
	var tmp1, tmp2 int64
	tmp1 = mongoinfo.startts << 32
	tmp2 = mongoinfo.stopts << 32
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
		logger.Println("MongoTimestamp:", result["ts"], "; UnixTimestamp:", timestamp)
		mongoinfo.ApplyOplog(result, result["ns"].(string))
	}

}

func main() {
	var fromhost, tohost, userName, passWord, database, collection string
	var startts, stopts int64
	var fromport, toport, logpath, slient string
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
	flag.StringVar(&slient, "slient", "no", "slient or not")

	flag.Parse()

	if fromhost != "" && tohost != "" && database != "" && logpath != "" && startts != 0 && stopts != 0 {

		logfile, err1 = os.OpenFile(logpath, os.O_RDWR|os.O_CREATE, 0666)
		defer logfile.Close()
		if err1 != nil {
			logger.Println(err1.Error())
			os.Exit(-1)
		}

		if slient == "yes" {

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
		mongoinfo := Newmongoinfo(fromhost, tohost, userName, passWord, database, collection, startts, stopts, fromport, toport)
		mongoinfo.StartRestore()
		logger.Println("=====Done.=====")

	} else {

		fmt.Println("Please use -help to check the usage")
		fmt.Println("At least need fromhost,tohost,database,logpath,startts and stopts")

	}

}
