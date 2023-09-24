package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"time"
)

var (
	GlobalNanoFlag = true
)

type TimeNTP uint64

type UnixOpts struct {
	nano    bool
	seconds bool
	unix    bool
	month   bool
	year    bool
	minutes bool
	day     bool
}

type packet struct {
	Key            uint8  // leap yr indicator, ver number, and mode
	Stratum        uint8  // stratum of local clock
	Poll           int8   // poll exponent
	Precision      int8   // precision exponent
	RootDelay      uint32 // root delay
	RootDispersion uint32 // root dispersion
	ReferenceID    uint32 // reference id
	RefTimeSec     uint32 // reference timestamp sec
	RefTimeFrac    uint32 // reference timestamp fractional
	OrigTimeSec    uint32 // origin time secs
	OrigTimeFrac   uint32 // origin time fractional
	RxTimeSec      uint32 // receive time secs
	RxTimeFrac     uint32 // receive time frac
	TxTimeSec      uint32 // transmit time secs
	TxTimeFrac     uint32 // transmit time frac
} // adapted from github.com/vladimirvivien/go-ntp-client

// var ntpSeventyYearOfset = ((70 * 365) + 17) * 86400 // eval at runtime
const ntpSeventyYearOffset = ((70 * 365) + 17) * 86400 // eval at compile time
const nanoPerSecond = 1e9                              // nanoseconds per second

func main() {
	address := "0.europe.pool.ntp.org:123"
	flag.StringVar(&address, "address", "0.europe.pool.ntp.org:123", "address of the ntp server. should be in 'host:server' format")
	flag.BoolVar(&GlobalNanoFlag, "globalnano", true, "nano format for time output.")
	flag.Parse()

	//timeYarn, err := ntp.Time("0.beevik-ntp.pool.ntp.org")
	//if &timeYarn != nil {
	//	fmt.Println(timeYarn)
	//	return
	//}

	// Setup a UDP connection
	conn, err := net.Dial("udp", address)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {

		}
	}(conn)

	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		log.Fatalf("failed to set deadline: %v", err)
	}

	// configure request settings by specifying the first byte as
	// 00 011 011 (or 0x1B)
	// |  |   +-- client mode (3)
	// |  + ----- version (3)
	// + -------- leap year indicator, 0 no warning // adapted from github.com/vladimirvivien/go-ntp-client
	req := &packet{Key: 0x1B}
	//req.Println()

	//save the packets into a struct
	transmitTime := time.Now()

	// send time request
	if err := binary.Write(conn, binary.BigEndian, req); err != nil {
		log.Fatalf("failed to send request: %v", err)
	}

	// block to receive server response
	resp := &packet{}
	if err := binary.Read(conn, binary.BigEndian, resp); err != nil {
		log.Fatalf("failed to read server response: %v", err)
	}

	var timeClientReceived interface{}
	if GlobalNanoFlag {
		timeClientReceived = time.Now().Add(time.Since(transmitTime)).UnixNano()
	} else {
		timeClientReceived = time.Now().Add(time.Since(transmitTime))
	}
	timeTest := time.Now().UnixMicro()

	fmt.Println(resp)

	fmt.Printf("FracPersec: %9.15f", float64(resp.RefTimeFrac/nanoPerSecond))

	if resp.OrigTimeSec <= 0 {
		//resp.OrigTimeSec = transmitTime.Second() //unix
		//resp.OrigTimeSec = Unix(uint32(transmitTime.Unix()), 0, &UnixOpts{nano: true}).(uint32)
		resp.OrigTimeSec = uint32(transmitTime.Unix() + ntpSeventyYearOffset)
		resp.OrigTimeFrac = uint32(transmitTime.Nanosecond())
	}

	resp.Println()
	resp.UnixOrigPrintln()
	resp.UnixRxPrintln()
	resp.UnixTxPrintln()
	fmt.Printf("Time response arrived from NTP Server\n%v\n\n", timeClientReceived)
	resp.UnixRefPrintln()

	resp.TimeOffsetPrintln(timeClientReceived)
	resp.RoundTripDelayPrintln(timeClientReceived)

	time.Sleep(time.Second * 5)

	fmt.Printf("\nTime Difference: %v", time.Now().Sub(time.UnixMicro(timeTest)))

}

func (p *packet) Println() {
	fmt.Println("\nRaw Response from NTP Server")
	fmt.Printf("Key: %x\nStratum: %v\nPoll: %v\nPrecision: %v\nRootDelay: %v\nRootDispersion: %v\nReferenceID: %v\nRefTimeSec: %v\nRefTimeFrac: %v\nOrigTimeSec: %v\nOrigTimeFrac: %v\nRxTimeSec: %v\nRxTimeFrac: %v\nTxTimeSec: %v\nTxTimeFrac: %v\n\n", p.Key, p.Stratum, p.Poll, p.Precision, p.RootDelay, p.RootDispersion, p.ReferenceID, p.RefTimeSec, p.RefTimeFrac, p.OrigTimeSec, p.OrigTimeFrac, p.RxTimeSec, p.RxTimeFrac, p.TxTimeSec, p.TxTimeFrac)
}

func (p *packet) TimeOffset(arrival interface{}) interface{} {
	origin, _, transmit, receive := p.ConvUnixAll(&UnixOpts{nano: GlobalNanoFlag})
	switch GlobalNanoFlag {
	case true:
		originInt, transmitInt, receiveInt, arrivalInt := origin.(int64), transmit.(int64), receive.(int64), arrival.(int64)
		return ((receiveInt - originInt) + (arrivalInt - transmitInt)) / 2
	case false:
		originInt, transmitInt, receiveInt, arrivalInt := origin.(time.Time), transmit.(time.Time), receive.(time.Time), arrival.(time.Time)
		//return ((receiveInt - originInt) + (arrivalInt - transmitInt)) / 2
		sub := (receiveInt.Sub(originInt) + arrivalInt.Sub(transmitInt)) / 2
		fmt.Println("TimeOffset sub (in duration):", sub)
		return sub
	}

	//return ((receiveInt - originInt) + (arrivalInt - transmitInt)) / 2
	return nil
}

func (p *packet) TimeOffsetPrintln(arrival interface{}) {
	switch GlobalNanoFlag {
	case true:
		fmt.Printf("Time Offset calculated: %v\n", p.TimeOffset(arrival).(int64))
	case false:
		fmt.Printf("Time Offset calculated: %v\n", p.TimeOffset(arrival).(time.Duration))
	}
}

func (p *packet) RoundTripDelay(arrival interface{}) interface{} {
	origin, _, transmit, receive := p.ConvUnixAll(&UnixOpts{nano: GlobalNanoFlag})
	switch GlobalNanoFlag {
	case true:
		originInt, transmitInt, receiveInt, arrivalInt := origin.(int64), transmit.(int64), receive.(int64), arrival.(int64)
		return (arrivalInt - originInt) - (transmitInt - receiveInt)
	case false:
		originInt, transmitInt, receiveInt, arrivalInt := origin.(time.Time), transmit.(time.Time), receive.(time.Time), arrival.(time.Time)
		//return ((receiveInt - originInt) + (arrivalInt - transmitInt)) / 2
		sub := (receiveInt.Sub(originInt) + arrivalInt.Sub(transmitInt)) / 2
		fmt.Println("RoundTripTime (in duration):", sub)
		return sub
	}

	return nil
}

func (p *packet) RoundTripDelayPrintln(arrival interface{}) {
	switch GlobalNanoFlag {
	case true:
		fmt.Printf("RoundTrip Delay calculated: %v\n", p.RoundTripDelay(arrival).(int64))
	case false:
		fmt.Printf("RoundTrip Delay calculated: %v\n", p.RoundTripDelay(arrival).(time.Duration))
	}
}

func (p *packet) ConvUnixAll(u *UnixOpts) (origin, ref, transmit, receive interface{}) {
	return Unix(p.OrigTimeSec, p.OrigTimeFrac, &UnixOpts{nano: GlobalNanoFlag}), Unix(p.RefTimeSec, p.RefTimeFrac, &UnixOpts{nano: GlobalNanoFlag}), Unix(p.TxTimeSec, p.TxTimeFrac, &UnixOpts{nano: GlobalNanoFlag}), Unix(p.RefTimeSec, p.RefTimeFrac, &UnixOpts{nano: GlobalNanoFlag})
}

func Unix(sec, frac uint32, u *UnixOpts) interface{} {
	secs := float64(sec) - ntpSeventyYearOffset // ntp time is being calc from 1900, this sets the prime time to be 1/1/1970
	nano := (int64(frac * 1e9)) >> 32           // reversing the conversion of nano to fraction: https://stackoverflow.com/questions/29112071/how-to-convert-ntp-time-to-unix-epoch-time-in-c-language-linux

	if u.nano {
		return time.Unix(int64(secs), nano).UnixNano()
	}

	return time.Unix(int64(secs), nano)
}

func (p *packet) UnixRefPrintln() {
	fmt.Println("TimeNow ReferenceTimestamp in Unix format")
	fmt.Printf("\nBefore: Sec: %v, Frac: %v\nAfter: %v\n\n", p.RefTimeSec, p.RefTimeFrac, Unix(p.RefTimeSec, p.RefTimeFrac, &UnixOpts{nano: GlobalNanoFlag}))
}

func (p *packet) UnixOrigPrintln() {
	fmt.Println("TimeNow OriginTimestamp in Unix format")
	fmt.Printf("Before: Sec: %v, Frac: %v\nAfter: %v\n", p.OrigTimeSec, p.OrigTimeFrac, Unix(p.OrigTimeSec, p.OrigTimeFrac, &UnixOpts{nano: GlobalNanoFlag}))
}

func (p *packet) UnixRxPrintln() {
	fmt.Println("TimeNow ReceiveTimestamp in Unix format")
	fmt.Printf("Before: Sec: %v, Frac: %v\nAfter: %v\n", p.RxTimeSec, p.RxTimeFrac, Unix(p.RxTimeSec, p.RxTimeFrac, &UnixOpts{nano: GlobalNanoFlag}))
}

func (p *packet) UnixTxPrintln() {
	fmt.Println("TimeNow TransmitTimestamp in Unix format")
	fmt.Printf("Before: Sec: %v, Frac: %v\nAfter: %v\n", p.TxTimeSec, p.TxTimeFrac, Unix(p.TxTimeSec, p.TxTimeFrac, &UnixOpts{nano: GlobalNanoFlag}))
}
