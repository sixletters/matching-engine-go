package main

import (
	"sort"
)

type Orderbook struct {
	instrument string
	ReqChannel chan Request
	orderIndex map[uint32]Request
	bids       map[uint32]*PriceLevel
	asks       map[uint32]*PriceLevel
}

func NewOrderBook(instrument string) *Orderbook {
	orderbook := &Orderbook{
		instrument: instrument,
		orderIndex: make(map[uint32]Request),
		bids:       make(map[uint32]*PriceLevel),
		asks:       make(map[uint32]*PriceLevel),
		ReqChannel: make(chan Request, 100),
	}
	go orderbook.obWorker()
	return orderbook
}

func (ob *Orderbook) handleRequest(req Request) {
	ob.ReqChannel <- req
}

// goroutine handling a single orderbook
func (ob *Orderbook) obWorker() {
	for {
		req := <-ob.ReqChannel
		if req.orderType == inputCancel {
			ob.cancelOrder(req)
		} else {
			inputPipe := make(chan (chan logData), 100)
			req := ob.fillOrder(req, inputPipe)
			if req.count > 0 {
				ob.addOrder(req, inputPipe)
			}
			go ob.orderLogger(inputPipe)
			close(inputPipe)
		}
	}
}

func (ob *Orderbook) orderLogger(inputPipe <-chan (chan logData)) {
	for {
		orderChan, exists := <-inputPipe
		if !exists {
			break
		}
		for {
			log, exists := <-orderChan
			if !exists {
				break
			}
			if log.getLogType() == logExecuted {
				executeLog := log.(executeLog)
				outputOrderExecuted(
					executeLog.restingId,
					executeLog.newId,
					executeLog.execId,
					executeLog.price,
					executeLog.count,
					executeLog.outTime,
				)
			} else {
				addLog := log.(addLog)
				outputOrderAdded(
					addLog.in,
					addLog.outTime,
				)
			}
		}
	}
}

func (ob *Orderbook) addOrder(req Request, inputPipe chan (chan logData)) {
	var orderMap map[uint32]*PriceLevel
	if req.orderType == inputBuy {
		orderMap = ob.bids
	} else {
		orderMap = ob.asks
	}
	var pl *PriceLevel
	pl, exists := orderMap[req.price]
	if !exists {
		pl = NewPriceLevel(req.orderType)
		orderMap[req.price] = pl
	}
	pl.TotalQuantity += req.count
	ob.orderIndex[req.orderId] = req
	outputchan := make(chan logData, 100)
	inputPipe <- outputchan
	pl.handleRequest(req, outputchan)
}

func (ob *Orderbook) fillOrder(req Request, inputPipe chan (chan logData)) Request {
	var orderMap map[uint32]*PriceLevel
	if req.orderType == inputBuy {
		orderMap = ob.asks
	} else {
		orderMap = ob.bids
	}
	keys := make([]uint32, 0, len(orderMap))
	for k := range orderMap {
		keys = append(keys, k)
	}
	if req.orderType == inputBuy {
		sort.SliceStable(keys, func(i, j int) bool {
			//ascending
			return keys[i] < keys[j]
		})
	} else {
		sort.SliceStable(keys, func(i, j int) bool {
			// descending
			return keys[i] > keys[j]
		})
	}
	for _, price := range keys {
		pl := orderMap[price]
		// if there is no more requests or if price cannot be matched, then we break.
		if req.count == 0 || !req.CanMatchPrice(price) {
			break
		}
		logchan := make(chan logData, 100)
		inputPipe <- logchan
		pl.handleRequest(req, logchan)
		// if request has more than current pl, then we can fill all orders at that pl
		fillQty := min(pl.TotalQuantity, req.count)
		pl.TotalQuantity -= fillQty
		req.count -= fillQty
	}
	return req
}

func (ob *Orderbook) cancelOrder(req Request) {
	reqInMem, exists := ob.orderIndex[req.orderId]
	if !exists {
		input := input{
			orderType:  req.orderType,
			orderId:    req.orderId,
			price:      req.price,
			count:      req.count,
			instrument: req.instrument,
		}
		outputOrderDeleted(input, false, req.timestamp)
		return
	}
	if reqInMem.orderType == inputBuy {
		ob.bids[reqInMem.price].handleRequest(req, nil)
	} else {
		ob.asks[reqInMem.price].handleRequest(req, nil)
	}
	delete(ob.orderIndex, req.orderId)
}
