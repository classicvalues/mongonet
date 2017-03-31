package main

import "flag"
import "fmt"
import "os"
import "time"

import "gopkg.in/mgo.v2/bson"

import "github.com/erh/mongonet"

const (
	errorCode     = 20000
	errorCodeName = "SNITesterError"
)

type MyFactory struct {
}

func (myf *MyFactory) NewInterceptor(ps *mongonet.ProxySession) (mongonet.ProxyInterceptor, error) {
	return &MyInterceptor{ps}, nil
}

type MyInterceptor struct {
	ps *mongonet.ProxySession
}

func (myi *MyInterceptor) sniResponse() mongonet.SimpleBSON {
	doc := bson.D{{"sniName", myi.ps.SSLServerName}, {"ok", 1}}
	raw, err := mongonet.SimpleBSONConvert(doc)
	if err != nil {
		panic(err)
	}
	return raw
}

func (myi *MyInterceptor) InterceptClientToMongo(m mongonet.Message) (mongonet.Message, mongonet.ResponseInterceptor, mongonet.MongoError) {
	switch mm := m.(type) {
	case *mongonet.QueryMessage:
		if !mongonet.NamespaceIsCommand(mm.Namespace) {
			return m, nil, mongonet.NoMongoError
		}

		query, err := mm.Query.ToBSOND()
		if err != nil || len(query) == 0 {
			// let mongod handle error message
			return m, nil, mongonet.NoMongoError
		}

		cmdName := query[0].Name
		if cmdName != "sni" {
			return m, nil, mongonet.NoMongoError
		}

		return nil, nil, newSNIError(myi.ps.RespondToCommand(mm, myi.sniResponse()))
	case *mongonet.CommandMessage:
		if mm.CmdName != "sni" {
			return mm, nil, mongonet.NoMongoError
		}
		return nil, nil, newSNIError(myi.ps.RespondToCommand(mm, myi.sniResponse()))
	}

	return m, nil, mongonet.NoMongoError
}

func (myi *MyInterceptor) Close() {
}
func (myi *MyInterceptor) TrackRequest(mongonet.MessageHeader) {
}
func (myi *MyInterceptor) TrackResponse(mongonet.MessageHeader) {
}

func (myi *MyInterceptor) CheckConnection() error {
	return nil
}

func (myi *MyInterceptor) ConnectionCheckInterval() time.Duration {
	return 0
}

func main() {

	bindHost := flag.String("host", "127.0.0.1", "what to bind to")
	bindPort := flag.Int("port", 9999, "what to bind to")
	mongoHost := flag.String("mongoHost", "127.0.0.1", "host mongo is on")
	mongoPort := flag.Int("mongoPort", 27017, "port mongo is on")

	flag.Parse()

	pc := mongonet.NewProxyConfig(*bindHost, *bindPort, *mongoHost, *mongoPort)

	pc.UseSSL = true
	if len(flag.Args()) < 2 {
		fmt.Printf("need to specify ssl cert and key\n")
		os.Exit(-1)
	}

	pc.SSLKeys = []mongonet.SSLPair{
		{flag.Arg(0), flag.Arg(1)},
	}

	pc.InterceptorFactory = &MyFactory{}

	pc.MongoSSLSkipVerify = true

	proxy := mongonet.NewProxy(pc)

	err := proxy.Run()
	if err != nil {
		panic(err)
	}
}

func newSNIError(err error) mongonet.MongoError {
	if err == nil {
		return mongonet.NoMongoError
	}

	return mongonet.NewMongoError(err, errorCode, errorCodeName)
}
