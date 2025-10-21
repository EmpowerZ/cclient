package cclient

import "github.com/EmpowerZ/fhttp/httptrace"

func traceConnectStart(trace *httptrace.ClientTrace, network, addr string) {
	if trace == nil || trace.ConnectStart == nil {
		return
	}
	trace.ConnectStart(network, addr)
}

func traceConnectDone(trace *httptrace.ClientTrace, network, addr string, err error) {
	if trace == nil || trace.ConnectDone == nil {
		return
	}
	trace.ConnectDone(network, addr, err)
}
