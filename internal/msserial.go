package msserial

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"time"

	"github.com/golang/glog"
	"github.com/tarm/serial"
)

func MakeSerialProducer(port string) chan []byte {

	dataChannel := make(chan []byte)

	go func() {
		for {
			glog.Infof("Opening serial port %s", port)
			c := &serial.Config{
				Name:        port,
				Baud:        115200,
				ReadTimeout: time.Second * 2,
			}
			serialPort, err := serial.OpenPort(c)
			if err != nil {
				glog.Error("Unable to open serial port, retrying in 2s: ", err)
				time.Sleep(time.Second * 2)
				continue
			}
			glog.Info("Successfully opened serial port")
			time.Sleep(time.Millisecond * 15)
			serialPort.Flush()

			for !CommunicationTest(serialPort) {
				glog.Error("Communication test failure, retrying in 2s")
				time.Sleep(time.Second * 2)
			}
			glog.Info("Communication test OK!")

			time.Sleep(time.Second * 2)
			errorAttempts := 0
			for {
				time.Sleep(time.Millisecond * 100)
				data, err := FetchRealtimeData(serialPort)
				if err != nil {
					glog.Error("Data fetch error: ", err)
					errorAttempts++
					if errorAttempts > 5 {
						break
					}
				}
				dataChannel <- data
			}

			glog.Warning("Serial failure, resetting serial connection...")
			serialPort.Close()
		}
	}()
	return dataChannel
}

func receiveBytes(port *serial.Port, expectedBytes int) ([]byte, error) {
	offset := 0
	responseBuffer := make([]byte, expectedBytes)

	for offset < expectedBytes {
		readBuffer := make([]byte, expectedBytes-offset)
		receivedBytes, err := port.Read(readBuffer)
		if err != nil {
			return nil, err
		}
		responseBuffer = append(responseBuffer[0:offset], readBuffer[0:receivedBytes]...)
		offset += receivedBytes
	}
	return responseBuffer, nil
}

func receiveFrame(port *serial.Port) ([]byte, error) {
	payloadSizeHeader, err := receiveBytes(port, 2)
	if err != nil {
		return nil, err
	}
	payloadSize := int(binary.BigEndian.Uint16(payloadSizeHeader))
	payload, err := receiveBytes(port, payloadSize)
	if err != nil {
		return nil, err
	}

	checksum, err := receiveBytes(port, 4)
	if err != nil {
		return nil, err
	}

	if len(payload) != payloadSize {
		return nil, fmt.Errorf("Invalid frame: data size expected to be %d received %d bytes", payloadSizeHeader, len(payload))
	}
	if crc32.ChecksumIEEE(payload) != binary.BigEndian.Uint32(checksum) {
		return nil, errors.New("Invalid frame: checksum error")
	}

	glog.V(3).Infof("Received %d payload bytes", len(payload))
	glog.V(3).Infof("Received payload: \n%s", hex.Dump(payload))

	return payload, nil

}

func sendCommand(port *serial.Port, data []byte) ([]byte, error) {
	sizeHeader := make([]byte, 2)
	binary.BigEndian.PutUint16(sizeHeader, uint16(len(data)))

	crcTrailer := make([]byte, 4)
	binary.BigEndian.PutUint32(crcTrailer, crc32.ChecksumIEEE(data))

	requestFrame := append(append(sizeHeader, data...), crcTrailer...)
	glog.V(3).Infof("Sending frame: \n%s", hex.Dump(requestFrame))

	_, err := port.Write(requestFrame)
	if err != nil {
		return nil, err
	}

	return receiveFrame(port)
}

// FetchRealtimeData - send the fetch realtime data command to an open serial port
func FetchRealtimeData(port *serial.Port) ([]byte, error) {
	data := []byte{
		'r',        // read command
		0x00,       // canID
		0x07,       // table
		0x00, 0x00, // offset
		0x00, 0xd4, // number of bytes to fetch (212 bytes full set)
	}
	return sendCommand(port, data)
}

// CommunicationTest - attempt to send the check command to the ECU true on successful response
func CommunicationTest(port *serial.Port) bool {
	data := []byte{'c'}
	_, err := sendCommand(port, data)
	if err != nil {
		glog.Error("Communication test error: ", err)
		return false
	}
	return true
}
