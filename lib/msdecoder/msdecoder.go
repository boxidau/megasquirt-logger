package msdecoder

import (
	"encoding/binary"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-ini/ini"
	"github.com/golang/glog"
)

type Decoder struct {
	config *ini.File
	// extractors    map[string]OutputChannel
	// datalogConfig []LogItem
}

type OutputChannel struct {
	Name      string
	Unit      string
	Extractor func(data []byte) float64
}

type LogItem struct {
	OutputChannelKey string
	FieldName        string
	FieldType        string
	Formatter        func(value float64) string
}

// New creates a Decoder for a config file
func New(configFile string) Decoder {
	config, err := ini.ShadowLoad(configFile)
	if err != nil {
		glog.Fatal("Error reading ini file ", err)
	}

	d := Decoder{config}
	return d
}

func (d *Decoder) LogItemExtractors() {
	datalogConfig := d.config.Section("Datalog").Key("entry").ValueWithShadows()

	glog.Info(datalogConfig)
}

// OutputChannelExtractors generates extractor functions and field info
func (d *Decoder) OutputChannelExtractors() (map[string]OutputChannel, error) {
	outputChannels, err := d.config.GetSection("OutputChannels")
	if err != nil {
		glog.Error("Failed to get OutputChannels section of config: ", err)
		return nil, err
	}

	cleanupRegex := regexp.MustCompile(`\{.*bitStringValue\(.*algorithmUnits.*\}`)

	extractors := make(map[string]OutputChannel, 0)
	for _, valueConfig := range outputChannels.Keys() {
		key := valueConfig.Name()
		if key == "time" {
			extractors[key] = OutputChannel{
				Name: key,
				Unit: "",
				Extractor: func([]byte) float64 {
					return float64(time.Now().Unix())
				},
			}
		}

		channelConfigFields := strings.Split(
			cleanupRegex.ReplaceAllString(valueConfig.String(), "kPa"),
			", ",
		)

		if channelConfigFields[0] == "scalar" && len(channelConfigFields) == 6 {
			extractors[key] = makeScalarExtractor(key, channelConfigFields)
		}
		if channelConfigFields[0] == "bits" && len(channelConfigFields) == 4 {
			extractors[key] = makeBitsetExtractor(key, channelConfigFields)
		}
	}
	return extractors, nil
}

func makeBitsetExtractor(key string, channelConfigFields []string) OutputChannel {
	outputChannelConfig := OutputChannel{
		Name: key,
		Unit: "",
	}
	encoding := channelConfigFields[1]
	offset, _ := strconv.ParseInt(channelConfigFields[2], 10, 64)
	// first byte is a flag field so increment offset by 1
	offset++

	bitOffsetRange := strings.SplitN(strings.Trim(channelConfigFields[3], "[]"), ":", 2)
	if bitOffsetRange[0] != bitOffsetRange[1] || encoding != "U08" {
		glog.Fatalf("Cannot handle bit ranges or non U08 packing, field: %s", key)
	}

	// bit offset from MSB
	bitOffset, _ := strconv.ParseInt(bitOffsetRange[0], 10, 64)
	outputChannelConfig.Extractor = func(data []byte) float64 {
		dataByte := uint8(data[offset])
		return float64((dataByte >> (7 - bitOffset)) & 1)
	}
	return outputChannelConfig
}

func makeScalarExtractor(key string, channelConfigFields []string) OutputChannel {
	outputChannelConfig := OutputChannel{
		Name: key,
		Unit: strings.Trim(channelConfigFields[3], "\""),
	}

	encoding := channelConfigFields[1]
	offset, _ := strconv.ParseInt(channelConfigFields[2], 10, 64)
	// first byte is a flag field so increment offset by 1
	offset++

	outputChannelConfig.Extractor = func(data []byte) float64 {
		var extractedValue int64 = 0

		switch encoding {
		case "S32":
			extractedValue = int64(int32(binary.BigEndian.Uint32(data[offset : offset+4])))
			break
		case "S16":
			extractedValue = int64(int16(binary.BigEndian.Uint16(data[offset : offset+2])))
			break
		case "S08":
			extractedValue = int64(int8(data[offset]))
			break
		case "U32":
			extractedValue = int64(binary.BigEndian.Uint32(data[offset : offset+4]))
			break
		case "U16":
			extractedValue = int64(binary.BigEndian.Uint16(data[offset : offset+2]))
			break
		case "U08":
			extractedValue = int64(uint8(data[offset]))
			break
		default:
			glog.Fatalf("Invalid encoding for field %s: %s", key, encoding)
		}

		multiplier, _ := strconv.ParseFloat(channelConfigFields[4], 64)
		scale, _ := strconv.ParseInt(channelConfigFields[5], 10, 64)

		// who the fuck would add before multiplying... megasquirt
		return float64(extractedValue+scale) * multiplier
	}

	return outputChannelConfig
}
