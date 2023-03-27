package main

type Side int

const (
	BUY  Side = 1
	SELL Side = 2
)

type Order struct {
	ID          uint32
	ClientID    uint32
	Price       uint32
	Side        Side
	Quantity    uint32
	ExecutionID uint32
}

func (o Order) CanMatchPrice(restingPrice uint32) bool {
	switch o.Side {
	case BUY:
		return o.Price >= restingPrice
	case SELL:
		return o.Price <= restingPrice
	default:
		panic("Invalid side")
	}
}
