package utils

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	httpHeaderServer      = "Server"
	httpHeaderContentType = "Content-Type"
	httpHeaderContentLen  = "Content-Length"
	httpHeaderConnection  = "Connection"
	ociServletPort        = 9015
	nodeHealthURL         = "http://127.0.0.1:9001/v1/cluster/nodehealth"
)

type installState int

const (
	unknown installState = iota
	installing
	finished
)

// OciRESTServlet provides REST controls for OCI Monitor
type OciRESTServlet struct {
	ociCtl *OciServiceControl
	lock   *sync.Mutex
	cli    *http.Client
	srv    *http.Server
	state  installState
}

// NewRESTServlet returns new instance of the OciRESTServlet
func NewRESTServlet(ctl *OciServiceControl) *OciRESTServlet {
	return &OciRESTServlet{
		ociCtl: ctl,
		lock:   &sync.Mutex{},
		cli: &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 2,
			},
			Timeout: 5 * time.Second,
		},
		state: unknown,
	}
}

// handleOciRest is a Servlet implementation passed to http.HandleFunc()
func (s *OciRESTServlet) handleOciRest(resp http.ResponseWriter, req *http.Request) {
	header := resp.Header()
	header.Add(httpHeaderServer, "Portworx/OCI-monitor v1.0")

	sendInvalidReq := func() {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		header.Add(httpHeaderConnection, "close")
	}

	switch req.Method {

	case http.MethodHead:
		s.handleGetHead(resp, false)

	case http.MethodGet:
		s.handleGetHead(resp, true)

	case http.MethodPost:
		var err error
		s.lock.Lock()
		defer s.lock.Unlock()
		if req.RequestURI == "/service/start" {
			err = s.ociCtl.Start()
		} else if req.RequestURI == "/service/stop" {
			err = s.ociCtl.Stop()
		} else if req.RequestURI == "/service/restart" {
			err = s.ociCtl.Restart()
		} else if req.RequestURI == "/service/enable" {
			err = s.ociCtl.Enable()
		} else if req.RequestURI == "/service/disable" {
			err = s.ociCtl.Disable()
		} else if req.RequestURI == "/service/remove" {
			err = s.ociCtl.Remove()
		} else {
			logrus.Warnf("Ignoring REST call %s %s", req.Method, req.RequestURI)
			sendInvalidReq()
			break
		}
		if err == nil {
			content := []byte(fmt.Sprintf("REST action%s completed successfully\n",
				strings.ToUpper(strings.Replace(req.RequestURI, "/", " ", -1))))
			header := resp.Header()
			header.Add(httpHeaderContentType, "text/plain")
			header.Add(httpHeaderContentLen, strconv.Itoa(len(content)))
			resp.WriteHeader(http.StatusOK)
			resp.Write(content)
		} else {
			logrus.WithError(err).Errorf("Error with REST call POST %s", req.RequestURI)
			http.Error(resp, err.Error(), http.StatusInternalServerError)
		}

	default:
		logrus.Warnf("Ignoring REST call %s %s", req.Method, req.RequestURI)
		sendInvalidReq()
	}
	s.flush(resp)
}

// handleGetHead method is shared between the GET/HEAD calls.
func (s *OciRESTServlet) handleGetHead(resp http.ResponseWriter, sendData bool) {

	sendResp := func(code int, content []byte) {
		if sendData {
			header := resp.Header()
			header.Add(httpHeaderContentType, "text/plain")
			header.Add(httpHeaderContentLen, strconv.Itoa(len(content)))
			resp.WriteHeader(code)
			resp.Write(content)
		} else {
			resp.WriteHeader(code)
		}
	}
	unknownStatusMsg := []byte("Node status UNKNOWN\n")

	// If we're not finished installing (or, unknown), send status and return immediately
	if st := s.getState(); st == unknown {
		sendResp(http.StatusServiceUnavailable, unknownStatusMsg)
		return
	} else if s.getState() != finished {
		sendResp(http.StatusServiceUnavailable, []byte("Node status INSTALLING\n"))
		return
	}

	// Else, proxy PX status
	pxResp, err := s.cli.Get(nodeHealthURL)
	defer func() {
		if pxResp != nil && pxResp.Body != nil {
			pxResp.Body.Close()
		}
	}()

	if err != nil {
		logrus.WithError(err).Warn("Could not retrieve PX node status")
		sendResp(http.StatusServiceUnavailable, unknownStatusMsg)
		if pxResp != nil && pxResp.Body != nil {
			logrus.Debug("Draining the invalid HTTP PX response") // .. so we can reuse keepalive session
			ioutil.ReadAll(pxResp.Body)
		}
	} else {
		content, err := ioutil.ReadAll(pxResp.Body)
		if err != nil {
			logrus.WithError(err).Warnf("Could not read PX node status")
		}
		sendResp(pxResp.StatusCode, content)
	}
}

// SetStateInstalling sets the OCI state to installing
func (s *OciRESTServlet) SetStateInstalling() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.state = installing
}

// SetStateInstallFinished sets the OCI state to finished installing
func (s *OciRESTServlet) SetStateInstallFinished() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.state = finished
}

// getState returns the OCI installing status
func (s *OciRESTServlet) getState() installState {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.state
}

// flush http-response implementation as suggested at
// http://stackoverflow.com/questions/19292113/not-buffered-http-responsewritter-in-golang
func (s *OciRESTServlet) flush(resp http.ResponseWriter) {
	if f, ok := resp.(http.Flusher); ok {
		f.Flush()
	}
}

// Start starts the OCI REST server
func (s *OciRESTServlet) Start() {
	if s.srv == nil {
		http.HandleFunc("/", s.handleOciRest)
		addr := fmt.Sprintf("127.0.0.1:%d", ociServletPort)
		s.srv = &http.Server{Addr: addr}
		go func() {
			if err := s.srv.ListenAndServe(); err != nil {
				logrus.WithError(err).Error("Could not start new HTTP server")
			}
		}()
	}
	// CHECKME is s.srv.Shutdown(nil) required?
}
