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
	// if pl.ReqChannel == nil {
	// 	go pl.plWorker()
	// }
	pl.ReqChannel <- req
	pl.logOutputChannel <- logChannel
}

func (pl *PriceLevel) plWorker() {
	for {
		req := <-pl.ReqChannel
		logger := <-pl.logOutputChannel
		if req.orderType == inputCancel {
			close(logger)
			_, exists := pl.OrderSet[req.orderId]
			if !exists {
				input := input{
					orderType:  req.orderType,
					orderId:    req.orderId,
					price:      req.price,
					count:      req.count,
					instrument: req.instrument,
				}
				outputOrderDeleted(input, false, req.timestamp)
				continue
			}
			// if request is type cancel, we iterate through and remove the order.
			for i, order := range pl.OrderQueue {
				if order.ID == req.orderId {
					pl.setQuantity(pl.getQuantity() - order.Quantity)
					// cxlOrder = *order
					if i == (len(pl.OrderQueue) - 1) {
						pl.OrderQueue = pl.OrderQueue[:i]
					} else {
						pl.OrderQueue = append(pl.OrderQueue[:i], pl.OrderQueue[i+1:]...)
					}
				}
				input := input{
					orderType:  req.orderType,
					orderId:    req.orderId,
					price:      req.price,
					count:      req.count,
					instrument: req.instrument,
				}
				delete(pl.OrderSet, req.orderId)
				outputOrderDeleted(input, true, req.timestamp)
			}
		} else {
			// if order type is the same as price level ordertype, we add the order.
			if req.orderType == pl.orderType {

				pl.addOrder(req, pl.OrderQueue, logger)
			} else {
				// we fill order if the price level ordertype is opposite of request.
				pl.fillOrder(req, pl.OrderQueue, logger)
			}
		}
	}
}

func (pl *PriceLevel) addOrder(req Request, orderQueue []*Order, outputchan chan logData) {
	newOrder := Order{
		ID:          req.orderId,
		Price:       req.price,
		Quantity:    req.count,
		ClientID:    req.clientID,
		Side:        BUY,
		ExecutionID: 1,
	}
	pl.OrderSet[req.orderId] = true
	pl.TotalQuantity += newOrder.Quantity
	input := input{
		orderType:  req.orderType,
		orderId:    req.orderId,
		price:      req.price,
		count:      req.count,
		instrument: req.instrument,
	}
	outputOrderAdded(input, req.timestamp)
	logData := addLog{
		logtype: logAdded,
		in:      input,
		outTime: req.timestamp,
	}
	outputchan <- logData
	close(outputchan)
	pl.OrderQueue = append(pl.OrderQueue, &newOrder)
}

func (pl *PriceLevel) fillOrder(req Request, orderQueue []*Order, outputchan chan logData) {
	qtyToFill := req.count
	// indexes we want to remove.
	toRemove := -1
	for i, order := range orderQueue {
		if qtyToFill >= order.Quantity {
			delete(pl.OrderSet, order.ID)
			outputOrderExecuted(order.ID, req.orderId, order.ExecutionID, order.Price, order.Quantity, req.timestamp)
			executionLog := executeLog{
				logtype:   logExecuted,
				restingId: order.ID,
				newId:     req.orderId,
				execId:    order.ExecutionID,
				price:     order.Price,
				count:     order.Quantity,
				outTime:   req.timestamp,
			}
			outputchan <- executionLog
			qtyToFill -= order.Quantity
			toRemove = i
		} else {
			outputOrderExecuted(order.ID, req.orderId, order.ExecutionID, order.Price, req.count, req.timestamp)
			executionLog := executeLog{
				logtype:   logExecuted,
				restingId: order.ID,
				newId:     req.orderId,
				execId:    order.ExecutionID,
				price:     order.Price,
				count:     req.count,
				outTime:   req.timestamp,
			}
			outputchan <- executionLog
			order.Quantity -= qtyToFill
			order.ExecutionID += 1
		}
		if qtyToFill <= 0 {
			break
		}
	}
	close(outputchan)
	orderQueue = orderQueue[toRemove+1:]
}
func (pl *PriceLevel) getQuantity() uint32 {
	return pl.TotalQuantity
}

func (pl *PriceLevel) setQuantity(quantity uint32) {
	pl.TotalQuantity = quantity
}
