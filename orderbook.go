package main

import (
	"sort"
)

type Orderbook struct {
	instrument string
	ReqChannel chan Request
	orderIndex map[uint32]Request
	buyOrder   map[uint32]*PriceLevel
	sellOrder  map[uint32]*PriceLevel
}

func NewOrderBook(instrument string) *Orderbook {
	orderbook := &Orderbook{
		instrument: instrument,
		orderIndex: make(map[uint32]Request),
		buyOrder:   make(map[uint32]*PriceLevel),
		sellOrder:  make(map[uint32]*PriceLevel),
		ReqChannel: make(chan Request),
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
			inputPipe := make(chan (chan logData), 1000)
			go ob.orderLogger(inputPipe)
			req, resting := ob.fillOrder(req, inputPipe)
			if resting {
				ob.addOrder(req, inputPipe)
			}
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
		orderMap = ob.buyOrder
	} else {
		orderMap = ob.sellOrder
	}
	var pl *PriceLevel
	pl, exists := orderMap[req.price]
	if !exists {
		pl = NewPriceLevel(req.orderType)
		orderMap[req.price] = pl
	}
	ob.orderIndex[req.orderId] = req
	outputchan := make(chan logData)
	inputPipe <- outputchan
	pl.handleRequest(req, outputchan)
}

func (ob *Orderbook) fillOrder(req Request, inputPipe chan (chan logData)) (Request, bool) {
	var orderMap map[uint32]*PriceLevel
	if req.orderType == inputBuy {
		orderMap = ob.sellOrder
	} else {
		orderMap = ob.buyOrder
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
		if req.count <= 0 || !req.CanMatchPrice(price) {
			break
		}
		logchan := make(chan logData, 1000)
		inputPipe <- logchan
		pl.handleRequest(req, logchan)
		// if request has more than current pl, then we can fill all orders at that pl
		if req.count > pl.getQuantity() {
			req.count = req.count - pl.getQuantity()
			pl.setQuantity(0)
		} else {
			pl.setQuantity(pl.getQuantity() - req.count)
			req.count = 0
		}
	}
	return req, req.count > 0

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
	uselessChan := make(chan logData)
	if exists {
		if reqInMem.orderType == inputBuy {
			ob.buyOrder[reqInMem.price].handleRequest(req, uselessChan)
		} else {
			ob.sellOrder[reqInMem.price].handleRequest(req, uselessChan)
		}
		delete(ob.orderIndex, req.orderId)
	}
}

type logData interface {
	getLogType() LogType
}

type LogType uint

const (
	logExecuted LogType = 0
	logAdded    LogType = 1
)

type executeLog struct {
	logtype   LogType
	restingId uint32
	newId     uint32
	execId    uint32
	price     uint32
	count     uint32
	outTime   int64
}

func (e executeLog) getLogType() LogType {
	return e.logtype
}

type addLog struct {
	logtype LogType
	in      input
	outTime int64
}

func (e addLog) getLogType() LogType {
	return e.logtype
}
