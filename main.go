package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/boxidau/megasquirt-logger/lib/msdecoder"
	"github.com/boxidau/megasquirt-logger/lib/msserial"
	"github.com/golang/glog"
)

var addr = flag.String("addr", ":8080", "WS server address")
var port = flag.String("port", "", "Serial port to comminicate with MS")
var configFile = flag.String("config-file", "config/mainController.ini", "Megasquirt INI file")

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

	decoder := msdecoder.New(*configFile)
	//outputChannels, _ := decoder.OutputChannelExtractors()
	decoder.LogItemExtractors()

	go func() {
		for data := range dataChan {
			// for _, outputChannel := range outputChannels {
			// 	//value := outputChannel.Extractor(data)
			// 	// glog.Infof(
			// 	// 	"Channel: %s\t\tValue: %.2f\t\tUnit: %s",
			// 	// 	outputChannel.Name, value, outputChannel.Unit,
			// 	// )
			// }
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
