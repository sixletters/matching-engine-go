package main

type PriceLevel struct {
	orderType        inputType
	TotalQuantity    uint32
	OrderQueue       []*Order
	OrderSet         map[uint32]bool
	ReqChannel       chan Request
	logOutputChannel chan (chan logData)
}

func NewPriceLevel(orderType inputType) *PriceLevel {
	pl := &PriceLevel{
		orderType:        orderType,
		OrderQueue:       make([]*Order, 0),
		OrderSet:         make(map[uint32]bool),
		ReqChannel:       make(chan Request, 100),
		logOutputChannel: make(chan chan logData, 100),
	}
	go pl.plWorker()
	return pl
}

func (pl *PriceLevel) handleRequest(req Request, logChannel chan logData) {
	pl.ReqChannel <- req
	pl.logOutputChannel <- logChannel
}

func (pl *PriceLevel) plWorker() {
	for {
		req := <-pl.ReqChannel
		logger := <-pl.logOutputChannel
		if req.orderType == inputCancel {
			pl.cancelOrder(req, logger)
		} else if req.orderType == pl.orderType {
			// if order type is the same as price level ordertype, we add the order.
			pl.addOrder(req, logger)
		} else {
			// we fill order if the price level ordertype is opposite of request.
			pl.fillOrder(req, logger)
		}
	}
}

func (pl *PriceLevel) cancelOrder(req Request, outputchan chan logData) {
	if reqInMem, exists := pl.OrderSet[req.orderId]; 

	if (!exists || reqInMem.client != req.client) {
		input := input{
			orderType:  req.orderType,
			orderId:    req.orderId,
			price:      req.price,
			count:      req.count,
			instrument: req.instrument,
		}
		outputOrderDeleted(input, false, req.timestamp)
		close(outputchan)
		return
	}

	// iterate and remove the order.
	for i, order := range pl.OrderQueue {
		if order.ID != req.orderId {
			continue
		}
		pl.OrderQueue = append(pl.OrderQueue[:i], pl.OrderQueue[i+1:]...)
		delete(pl.OrderSet, req.orderId)
		pl.TotalQuantity -= order.Quantity
		input := input{
			orderType:  req.orderType,
			orderId:    req.orderId,
			price:      req.price,
			count:      req.count,
			instrument: req.instrument,
		}
		outputOrderDeleted(input, true, req.timestamp)
		break
	}
	close(outputchan)
}

func (pl *PriceLevel) addOrder(req Request, outputchan chan logData) {
	newOrder := Order{
		ID:          req.orderId,
		Price:       req.price,
		Quantity:    req.count,
		ClientID:    req.clientID,
		Side:        BUY,
		ExecutionID: 1,
	}
	pl.OrderSet[req.orderId] = true
	input := input{
		orderType:  req.orderType,
		orderId:    req.orderId,
		price:      req.price,
		count:      req.count,
		instrument: req.instrument,
	}
	logData := addLog{
		logtype: logAdded,
		in:      input,
		outTime: req.timestamp,
	}
	pl.OrderQueue = append(pl.OrderQueue, &newOrder)
	outputchan <- logData
	close(outputchan)
}

func (pl *PriceLevel) fillOrder(req Request, outputchan chan logData) {
	// indexes we want to remove.
	toRemove := -1
	for i, order := range pl.OrderQueue {
		fillQty := min(req.count, order.Quantity)
		order.Quantity -= fillQty
		req.count -= fillQty

		executionLog := executeLog{
			logtype:   logExecuted,
			restingId: order.ID,
			newId:     req.orderId,
			execId:    order.ExecutionID,
			price:     order.Price,
			count:     fillQty,
			outTime:   req.timestamp,
		}
		outputchan <- executionLog

		if (order.Quantity == 0) {
			delete(pl.OrderSet, order.ID)
			toRemove = i
		} else {
			order.ExecutionID += 1
		}

		if req.count == 0 {
			break
		}
	}
	close(outputchan)
	pl.OrderQueue = pl.OrderQueue[toRemove+1:]
	// fmt.Println(pl.OrderQueue)
}
