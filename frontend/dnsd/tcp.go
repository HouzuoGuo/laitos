package dnsd

import (
	"fmt"
	"net"
	"strings"
	"time"
)

func (dnsd *DNSD) HandleTCPQuery(clientConn net.Conn) {
	defer clientConn.Close()
	// Check address against rate limit
	clientIP := clientConn.RemoteAddr().String()[:strings.LastIndexByte(clientConn.RemoteAddr().String(), ':')]
	if !dnsd.RateLimit.Add(clientIP, true) {
		return
	}
	// Check address against allowed IP prefixes
	var prefixOK bool
	for _, prefix := range dnsd.AllowQueryIPPrefixes {
		if strings.HasPrefix(clientIP, prefix) {
			prefixOK = true
			break
		}
	}
	if !prefixOK {
		dnsd.Logger.Printf("HandleTCPQuery", clientIP, nil, "client IP is not allowed by configuration")
		return
	}
	// Read query length
	clientConn.SetDeadline(time.Now().Add(IOTimeoutSec * time.Second))
	queryLenBuf := make([]byte, 2)
	_, err := clientConn.Read(queryLenBuf)
	if err != nil {
		dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error when reading length from client")
		return
	}
	queryLen := int(queryLenBuf[0])*256 + int(queryLenBuf[1])
	// Read query
	if queryLen > MaxPacketSize || queryLen < 1 {
		dnsd.Logger.Printf("HandleTCPQuery", clientIP, nil, "bad query length from client")
		return
	}
	queryBuf := make([]byte, queryLen)
	_, err = clientConn.Read(queryBuf)
	if err != nil {
		dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error when reading query from client")
		return
	}
	domainName := ExtractDomainName(queryBuf)
	// Formulate response
	var responseLen int
	var responseLenBuf []byte
	var responseBuf []byte
	var doForward bool
	if domainName == "" {
		// If I cannot figure out what domain is from the query, simply forward it without much concern.
		dnsd.Logger.Printf("HandleTCPQuery", clientIP, nil, "let forwarder handle non-name query")
		doForward = true
	} else {
		// This is a domain name query, check the name against black list and then forward.
		dnsd.BlackListMutex.Lock()
		_, blacklisted := dnsd.BlackList[domainName]
		dnsd.BlackListMutex.Unlock()
		if blacklisted {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, nil, "answer to black-listed domain \"%s\"", domainName)
			responseBuf = RespondWith0(queryBuf)
			responseLen = len(responseBuf)
			responseLenBuf = make([]byte, 2)
			responseLenBuf[0] = byte(responseLen / 256)
			responseLenBuf[1] = byte(responseLen % 256)
		} else {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, nil, "let forwarder handle domain \"%s\"", domainName)
			doForward = true
		}
	}
	// If queried domain is not black listed, forward the query to forwarder.
	if doForward {
		myForwarder, err := net.DialTimeout("tcp", dnsd.TCPForwardTo, IOTimeoutSec*time.Second)
		if err != nil {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error when connecting to forwarder")
			return
		}
		defer myForwarder.Close()
		// Send original query to forwarder without modification
		myForwarder.SetDeadline(time.Now().Add(IOTimeoutSec * time.Second))
		combinedBuf := make([]byte, len(queryLenBuf)+len(queryBuf))
		copy(combinedBuf[0:2], queryLenBuf)
		copy(combinedBuf[2:], queryBuf)
		if _, err = myForwarder.Write(combinedBuf); err != nil {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error when writing length to forwarder")
			return
		} else if _, err = myForwarder.Write(queryBuf); err != nil {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error when writing query to forwarder")
			return
		}
		// Retrieve forwarder's response
		responseLenBuf = make([]byte, 2)
		if _, err = myForwarder.Read(responseLenBuf); err != nil {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error when reading length from forwarder")
			return
		}
		responseLen = int(responseLenBuf[0])*256 + int(responseLenBuf[1])
		if responseLen > MaxPacketSize || responseLen < 1 {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, nil, "bad response length from forwarder")
			return
		}
		responseBuf = make([]byte, responseLen)
		if _, err = myForwarder.Read(responseBuf); err != nil {
			dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error when reading from forwarder")
			return
		}
	}
	// Send response to my client
	if _, err = clientConn.Write(responseLenBuf); err != nil {
		dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error when writing length to client")
		return
	} else if _, err = clientConn.Write(responseBuf); err != nil {
		dnsd.Logger.Printf("HandleTCPQuery", clientIP, err, "IO error when writing response to client")
		return
	}
	return
}

/*
You may call this function only after having called Initialise()!
Start DNS daemon to listen on TCP port only. Block caller.
*/
func (dnsd *DNSD) StartAndBlockTCP() error {
	listenAddr := fmt.Sprintf("%s:%d", dnsd.TCPListenAddress, dnsd.TCPListenPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	dnsd.Logger.Printf("StartAndBlockTCP", listenAddr, nil, "going to listen for queries")
	for {
		clientConn, err := listener.Accept()
		if err != nil {
			return err
		}
		go dnsd.HandleTCPQuery(clientConn)
	}

}
