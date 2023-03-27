package main

import "C"
import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
)

type Request struct {
	orderType  inputType
	orderId    uint32
	price      uint32
	count      uint32
	instrument string
	timestamp  int64
	clientID   uint32
}

func (r Request) CanMatchPrice(restingPrice uint32) bool {
	switch r.orderType {
	case inputBuy:
		return r.price >= restingPrice
	case inputSell:
		return r.price <= restingPrice
	default:
		panic("Invalid side")
	}
}

var clientID uint32 = 0

var OrderBookIndex map[string]*Orderbook = map[string]*Orderbook{}
var reqInstrumentMap map[uint32]string = map[uint32]string{}
var t int64 = 0

type Engine struct {}

func (e *Engine) accept(ctx context.Context, conn net.Conn) {
	clientID += 1
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	go e.handleConn(conn)
}

func (e *Engine) handleConn(conn net.Conn) {
	defer conn.Close()
	for {
		in, err := readInput(conn)
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			}
			return
		}
		// create request here
		t++
		req := Request{in.orderType, in.orderId, in.price, in.count, in.instrument, t, clientID}
		// if req is cancel, check if id -> instr map existence.
		if req.orderType == inputCancel {
			instrument, exists := reqInstrumentMap[req.orderId]
			if !exists {
				outputOrderDeleted(input{orderId: req.orderId}, false, t)
				continue
			}
			req.instrument = instrument
		} else {
			reqInstrumentMap[req.orderId] = req.instrument
		}
		if _, exists := OrderBookIndex[req.instrument]; !exists {
			OrderBookIndex[in.instrument] = NewOrderBook(req.instrument)
		}
		OrderBookIndex[req.instrument].handleRequest(req)
	}
}

// cleanup function
func (e *Engine) close() {
	// for _, channel := range ORDERBOOK_CHANNELS {
	// 	close(channel)
	// }
}
