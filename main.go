package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/beevik/ntp" //optional: on flag
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

// TODO Notes: Fix why the Offset and RTT is too large. sample below:
//&{28 2 0 -25 9 44 3650272056 3904557432 1899285837 0 0 3904558069 2690991889 3904558069 2691072910}
//TimeNow OriginTimestamp in Unix format
//Before: Sec: 3904558069, Frac: 555061000
//After: 2023-09-24 16:27:49 +0100 WEST
//TimeNow ReceiveTimestamp in Unix format
//Before: Sec: 3904558069, Frac: 2690991889
//After: 2023-09-24 16:27:49 +0100 WEST
//TimeNow TransmitTimestamp in Unix format
//Before: Sec: 3904558069, Frac: 2691072910
//After: 2023-09-24 16:27:49 +0100 WEST
//Time response arrived from NTP Server
//2023-09-24 16:27:49.555063084 +0100 WEST m=+0.218389127
//
//TimeNow ReferenceTimestamp in Unix format
//Before: Sec: 3904557432, Frac: 1899285837
//After: 2023-09-24 16:17:12 +0100 WEST
//
//TimeOffset sub (in duration): -5m18.222468458s
//Time Offset calculated: -5m18.222468458s
//RoundTripTime (in duration): -5m18.222468458s
//RoundTrip Delay calculated: -5m18.222468458s

var (
	GlobalNanoFlag        = true
	RefFrac        uint32 = 0
	RefDiff               = false
	TestBeevik            = false
	output                = &OutputResponse{}
)

var timeClientReceived interface{}

var (
	UTCOutput = `
---------------------------------------------------------------------------------
|  Field   |          Before           |         Time Output (UTC)              |
|          -----------------------------																				|
|          |  Seconds    |  Fraction   |                                        |
|          -----------------------------																				|
| Origin   | %v  | %v   | %v         |
| Receive  | %v  | %v   | %v         |
| Transmit | %v  | %v   | %v         |
| Dest     | %v  | %v   | %v         |
|-------------------------------------------------------------------------------|
| Ref      | %v  | %v   | %v         |
|-------------------------------------------------------------------------------|

---------------------------------------------------------------------------------
| Round Trip Delay (RTT) ==>	 %v    														      |
|-------------------------------------------------------------------------------|
| Time Offset						 ==>	 %v    														      |
|-------------------------------------------------------------------------------|
`
	NanoOutput = `
---------------------------------------------------------------------------------
|  Field   |          Before           |         Time Output (nano)             |
|          -----------------------------																				|
|          |  Seconds    |  Fraction   |                                        |
|          -----------------------------																				|
| Origin   | 3904559550  | 696334000   | 2023-09-24 16:52:30 +0100 WEST         |
| Receive  | 3904559550  | 696334000   | 2023-09-24 16:52:30 +0100 WEST         |
| Transmit | 3904559550  | 696334000   | 2023-09-24 16:52:30 +0100 WEST         |
| Dest     | 3904559550  | 696334000   | 1695571316000000000                    |
|-------------------------------------------------------------------------------|
| Ref      | 3904559550  | 696334000   | 2023-09-24 16:52:30 +0100 WEST         |
|-------------------------------------------------------------------------------|

---------------------------------------------------------------------------------
| Round Trip Delay (RTT) ==>	 424576208ns    														      |
|-------------------------------------------------------------------------------|
| Time Offset						 ==>	 424576208    															      |
|-------------------------------------------------------------------------------|
`
)

type TimeNTP uint64

//type DestinationTimeset struct {
//	Raw int64
//	String int
//}

type timeReceived struct {
	integer    int64
	nativeTime time.Time
}

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

// OutputResponse is used to hold the response packet for printing in a pretty format
type OutputResponse struct {
	OriginSec       string
	OriginFrac      string
	OriginComp      string // OriginComp holds the Origin time in the UTC/NTP time format
	ReceiveSec      string
	ReceiveFrac     string
	ReceiveComp     string // ReceiveComp holds the Receive time in the UTC/NTP time format
	TransmitSec     string
	TransmitFrac    string
	TransmitComp    string // TransmitComp holds the Transmit time in the UTC/NTP time format
	DestinationSec  string
	DestinationFrac string
	DestinationComp string // DestinationComp holds the Destination time in the UTC/NTP time format
	ReferenceSec    string
	ReferenceFrac   string
	ReferenceComp   string // ReferenceComp holds the Reference time in the UTC/NTP time format
	RTT             interface{}
	TimeOffset      interface{}
}

// var ntpSeventyYearOfset = ((70 * 365) + 17) * 86400 // eval at runtime
const ntpSeventyYearOffset = ((70 * 365) + 17) * 86400 // eval at compile time
const nanoPerSecond = 1e9                              // nanoseconds per second

func main() {
	address := "0.europe.pool.ntp.org:123"
	flag.StringVar(&address, "address", "0.europe.pool.ntp.org:123", "address of the ntp server. should be in 'host:server' format")
	flag.BoolVar(&GlobalNanoFlag, "globalnano", false, "nano format for time output.")
	flag.BoolVar(&RefDiff, "only-ref-diff", false, "only calculate time between reference time changes.")
	flag.BoolVar(&TestBeevik, "with-beevik", false, "run ntp using beevik ntp package.") // source: https://github.com/beevik/ntp
	flag.Parse()

	if TestBeevik {
		timeYarn, err := ntp.Time("0.beevik-ntp.pool.ntp.org")
		if err != nil {
			fmt.Println("TestBeevik dependent package is unavailable")
		}
		fmt.Println(timeYarn)
		os.Exit(0)
	}
	//os.Exit(0)

	// Setup a UDP connection
	conn := setUpConn(address)
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

	if err := binary.Write(conn, binary.BigEndian, req); err != nil {
		log.Fatalf("failed to send request: %v", err)
	}
	transmitTime := time.Now()
	//fmt.Println("time request left:", transmitTime.UnixNano())

	resp := &packet{}
	// block to receive server response
	if err := binary.Read(conn, binary.BigEndian, resp); err != nil {
		log.Fatalf("failed to read server response: %v", err)
	}

	//save the packets into a struct

	if RefDiff {
		if CalcRefDiff(address, req) {
			return
		}
	}

	output = &OutputResponse{}

	//fmt.Println(resp)

	if resp.OrigTimeSec <= 0 {
		//resp.OrigTimeSec = transmitTime.Second() //unix
		//resp.OrigTimeSec = Unix(uint32(transmitTime.Unix()), 0, &UnixOpts{nano: true}).(uint32)
		resp.OrigTimeSec = uint32(transmitTime.Unix() + ntpSeventyYearOffset)
		resp.OrigTimeFrac = uint32(transmitTime.Nanosecond()) // I'm making the assumption that the amount of nanoseconds stays the same regardless of the seventy-year offset (since it is all under 1 second)
		//fmt.Println("OriginTime: ", resp.OrigTimeSec, resp.OrigTimeFrac)
	}

	//resp.Println()
	//resp.UnixOrigPrintln()
	output.OriginSec, output.OriginFrac = stringInt32(resp.OrigTimeSec), stringInt32(resp.OrigTimeFrac)
	//resp.UnixRxPrintln()
	output.ReceiveSec, output.ReceiveFrac = stringInt32(resp.RxTimeSec), stringInt32(resp.RxTimeFrac)
	//resp.UnixTxPrintln()
	output.TransmitSec, output.TransmitFrac = stringInt32(resp.TxTimeSec), stringInt32(resp.TxTimeFrac)
	//resp.UnixRefPrintln()
	output.ReferenceSec, output.ReferenceFrac = stringInt32(resp.RefTimeSec), stringInt32(resp.RefTimeFrac)
	timeClientReceived = time.Now().Add(time.Since(transmitTime))
	//fmt.Printf("Time response arrived from NTP Server: %v\n\n", timeClientReceived.(time.Time).UnixNano())
	output.DestinationSec, output.DestinationFrac = stringInt32(uint32(timeClientReceived.(time.Time).Unix()+ntpSeventyYearOffset)), stringInt32(uint32(timeClientReceived.(time.Time).Nanosecond()))

	resp.TimeOffsetPrintln(timeClientReceived)
	resp.RoundTripDelayPrintln(timeClientReceived)
	resp.LoadTimesUnix()

	//fmt.Printf("%+v", output)

	output.Println()

	//time.Sleep(time.Second * 5)
	//
	//fmt.Printf("\nTime Difference: %v", time.Now().Sub(time.UnixMicro(timeTest)))

}

func CalcRefDiff(address string, req *packet) bool {
	timeLastRef := time.Now()
	pollCounter := 0
	defer fmt.Println(pollCounter)
	for {
		// send time request
		conn := setUpConn(address)
		//defer func(conn net.Conn) {
		//	err := conn.Close()
		//	if err != nil {
		//
		//	}
		//}(conn)

		if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
			log.Fatalf("failed to set deadline: %v", err)
		}

		if err := binary.Write(conn, binary.BigEndian, req); err != nil {
			log.Fatalf("failed to send request: %v", err)
		}

		resp := &packet{}
		// block to receive server response
		if err := binary.Read(conn, binary.BigEndian, resp); err != nil {
			log.Fatalf("failed to read server response: %v", err)
		}
		pollCounter++

		if resp.RefTimeFrac != RefFrac {
			fmt.Printf("Reference time has changed\nTime since last refTime (%v -> %v) at time (%v -> %v): %v\n", RefFrac, resp.RefTimeFrac, timeLastRef, time.Now(), time.Now().Sub(timeLastRef))
			RefFrac = resp.RefTimeFrac
			timeLastRef = time.Now().Add(time.Now().Sub(timeLastRef))
		}

		err := conn.Close()
		if err != nil {
			return true
		}

		time.Sleep(3 * time.Second)

	}
	return false
}

func setUpConn(address string) net.Conn {
	conn, err := net.Dial("udp", address)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	return conn
}

func (p *packet) Println() {
	fmt.Println("\nRaw Response from NTP Server")
	fmt.Printf("Key: %x\nStratum: %v\nPoll: %v\nPrecision: %v\nRootDelay: %v\nRootDispersion: %v\nReferenceID: %v\nRefTimeSec: %v\nRefTimeFrac: %v\nOrigTimeSec: %v\nOrigTimeFrac: %v\nRxTimeSec: %v\nRxTimeFrac: %v\nTxTimeSec: %v\nTxTimeFrac: %v\n\n", p.Key, p.Stratum, p.Poll, p.Precision, p.RootDelay, p.RootDispersion, p.ReferenceID, p.RefTimeSec, p.RefTimeFrac, p.OrigTimeSec, p.OrigTimeFrac, p.RxTimeSec, p.RxTimeFrac, p.TxTimeSec, p.TxTimeFrac)
}

func (p *packet) TimeOffset(arrival interface{}) interface{} {
	origin, _, transmit, receive := p.ConvUnixAll(&UnixOpts{nano: GlobalNanoFlag})
	switch GlobalNanoFlag {
	case true:
		//fmt.Println("origin:", origin)
		//fmt.Println("receive:", receive)
		//fmt.Println("transmit:", transmit)
		//fmt.Println("arrived:", arrival)
		originInt, transmitInt, receiveInt, arrivalInt := origin.(int64), transmit.(int64), receive.(int64), arrival.(time.Time).UnixNano()
		//fmt.Printf("((%v - %v) + (%v - %v)) / 2\n", receiveInt, originInt, arrivalInt, transmitInt)
		sub := ((receiveInt - originInt) + (arrivalInt - transmitInt)) / 2
		//fmt.Println("TimeOffset sub (in duration):", sub)
		output.TimeOffset = sub
		return sub
	case false:
		originInt, transmitInt, receiveInt, arrivalInt := origin.(time.Time), transmit.(time.Time), receive.(time.Time), arrival.(time.Time)
		//return ((receiveInt - originInt) + (arrivalInt - transmitInt)) / 2
		sub := (receiveInt.Sub(originInt) + arrivalInt.Sub(transmitInt)) / 2
		//fmt.Println("TimeOffset sub (in duration):", sub)
		output.TimeOffset = sub
		return sub
	}

	//return ((receiveInt - originInt) + (arrivalInt - transmitInt)) / 2
	return nil
}

func (p *packet) TimeOffsetPrintln(arrival interface{}) {
	switch GlobalNanoFlag {
	case true:
		fmt.Printf("Time Offset calculated: %v\n", p.TimeOffset(arrival))
	case false:
		fmt.Printf("Time Offset calculated: %v\n", p.TimeOffset(arrival))
	}
}

func (p *packet) RoundTripDelay(arrival interface{}) interface{} {
	origin, _, transmit, receive := p.ConvUnixAll(&UnixOpts{nano: GlobalNanoFlag})
	switch GlobalNanoFlag {
	case true:
		originInt, transmitInt, receiveInt, arrivalInt := origin.(int64), transmit.(int64), receive.(int64), arrival.(time.Time).UnixNano()
		delay := (arrivalInt - originInt) - (transmitInt - receiveInt)
		output.RTT = delay
		return delay
	case false:
		originInt, transmitInt, receiveInt, arrivalInt := origin.(time.Time), transmit.(time.Time), receive.(time.Time), arrival.(time.Time)
		//return ((receiveInt - originInt) + (arrivalInt - transmitInt))
		delay := arrivalInt.Sub(originInt) - transmitInt.Sub(receiveInt)
		//fmt.Println("RoundTripTime (in duration):", delay)
		output.RTT = delay
		return delay
	}

	return nil
}

func (p *packet) RoundTripDelayPrintln(arrival interface{}) {
	switch GlobalNanoFlag {
	case true:
		fmt.Printf("RoundTrip Delay calculated: %v\n", p.RoundTripDelay(arrival))
	case false:
		fmt.Printf("RoundTrip Delay calculated: %v\n", p.RoundTripDelay(arrival).(time.Duration))
	}
}

func (p *packet) ConvUnixAll(u *UnixOpts) (origin, ref, transmit, receive interface{}) {
	return Unix(p.OrigTimeSec, p.OrigTimeFrac, &UnixOpts{nano: GlobalNanoFlag}), Unix(p.RefTimeSec, p.RefTimeFrac, &UnixOpts{nano: GlobalNanoFlag}), Unix(p.TxTimeSec, p.TxTimeFrac, &UnixOpts{nano: GlobalNanoFlag}), Unix(p.RxTimeSec, p.RxTimeFrac, &UnixOpts{nano: GlobalNanoFlag})
}

// convert ntp time format to unix format (1900 - 1970) -- https://datatracker.ietf.org/doc/html/rfc5905?utm_source=substack&utm_medium=email#page-19:~:text=not%20been%20verified.%0A%20*/-,%23define%20JAN_1970,-2208988800UL%20/*%201970%20%2D%201900
func Unix(sec, frac uint32, u *UnixOpts) interface{} {
	secs := float64(sec) - ntpSeventyYearOffset // ntp time is being calc from 1900, this sets the prime time to be 1/1/1970
	nano := (int64(frac * nanoPerSecond)) >> 32 // reversing the conversion of nano to fraction: https://stackoverflow.com/questions/29112071/how-to-convert-ntp-time-to-unix-epoch-time-in-c-language-linux

	if u.nano {
		return time.Unix(int64(secs), nano).UnixNano()
	}

	return time.Unix(int64(secs), nano)
}

func (p *packet) LoadTimesUnix() {
	output.OriginComp = fmt.Sprintf("%d", Unix(p.OrigTimeSec, p.OrigTimeFrac, &UnixOpts{nano: GlobalNanoFlag}))
	output.ReferenceComp = fmt.Sprintf("%d", Unix(p.RefTimeSec, p.RefTimeFrac, &UnixOpts{nano: GlobalNanoFlag}))
	output.ReceiveComp = fmt.Sprintf("%d", Unix(p.RxTimeSec, p.RxTimeFrac, &UnixOpts{nano: GlobalNanoFlag}))
	output.TransmitComp = fmt.Sprintf("%d", Unix(p.TxTimeSec, p.TxTimeFrac, &UnixOpts{nano: GlobalNanoFlag}))
	output.DestinationComp = fmt.Sprintf("%d", Unix(uint32(timeClientReceived.(time.Time).Unix()+ntpSeventyYearOffset), uint32(timeClientReceived.(time.Time).Nanosecond()), &UnixOpts{nano: GlobalNanoFlag}))
}

func (p *packet) UnixRefPrintln() {
	fmt.Println("TimeNow ReferenceTimestamp in Unix format")
	fmt.Printf("Before: Sec: %v, Frac: %v\nAfter: %v\n\n", p.RefTimeSec, p.RefTimeFrac, Unix(p.RefTimeSec, p.RefTimeFrac, &UnixOpts{nano: GlobalNanoFlag}))
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

func (o *OutputResponse) Println() {
	fmt.Printf(UTCOutput, o.OriginSec, o.OriginFrac, o.OriginComp, o.ReceiveSec, o.ReceiveFrac, o.ReceiveComp, o.TransmitSec, o.TransmitFrac, o.TransmitComp, o.DestinationSec, o.DestinationFrac, o.DestinationComp, o.ReferenceSec, o.ReferenceFrac, o.ReferenceComp, o.RTT, o.TimeOffset)
}

func stringInt32(i uint32) string { return strconv.FormatInt(int64(i), 10) }
