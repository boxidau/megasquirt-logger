package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	msserial "github.com/boxidau/megasquirt-logger/internal"
	"github.com/golang/glog"
)

var addr = flag.String("addr", ":8080", "WS server address")
var port = flag.String("port", "", "Serial port to comminicate with MS")

func usage() {
	flag.PrintDefaults()
	os.Exit(2)
}

func init() {
	flag.Usage = usage
	flag.Set("logtostderr", "true")
	flag.Parse()
}

func main() {
	glog.Info("Starting logger...")

	outputStr := ""
	dataChan := msserial.MakeSerialProducer(*port)

	go func() {
		for data := range dataChan {
			outputStr = hex.Dump(data)
		}
	}()

	// webserver for debugging active frame
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s", outputStr)
	})
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
