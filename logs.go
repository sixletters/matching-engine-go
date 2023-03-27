package main

type logData interface {
	getLogType() LogType
}

type LogType uint8

const (
	logExecuted LogType = 1
	logAdded    LogType = 2
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