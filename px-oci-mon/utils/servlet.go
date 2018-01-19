package utils

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"k8s.io/api/core/v1"
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
	defaultOciEndpoint    = "127.0.0.1:9015"
	nodeHealthURL         = "http://127.0.0.1:9001/v1/cluster/nodehealth"
	svcUriPrefix          = "/service/"
	svcUriPrefixLen       = len(svcUriPrefix)
)

type installState int

const (
	unknown installState = iota
	installing
	finished
)

// OciRESTServlet provides REST controls for OCI Monitor
type OciRESTServlet struct {
	ociCtl      *OciServiceControl
	lock        *sync.Mutex
	cli         *http.Client
	srv         *http.Server
	state       installState
	node        *v1.Node
	errorsGrace *time.Time
}

// NewRESTServlet returns new instance of the OciRESTServlet
func NewRESTServlet(ctl *OciServiceControl, node *v1.Node) *OciRESTServlet {
	grace := time.Now().Add(60 * time.Second)
	return &OciRESTServlet{
		ociCtl: ctl,
		lock:   &sync.Mutex{},
		cli: &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 2,
			},
			Timeout: 5 * time.Second,
		},
		state:       unknown,
		node:        node,
		errorsGrace: &grace,
	}
}

// handleOciRest is a Servlet implementation passed to http.HandleFunc()
func (s *OciRESTServlet) handleOciRest(resp http.ResponseWriter, req *http.Request) {
	header := resp.Header()
	header.Add(httpHeaderServer, "Portworx/OCI-monitor v1.0")

	sendInvalidReq := func() {
		logrus.Warnf("Ignoring REST call %s %s", req.Method, req.RequestURI)
		resp.WriteHeader(http.StatusMethodNotAllowed)
		header.Add(httpHeaderConnection, "close")
	}

	switch req.Method {

	case http.MethodHead:
		s.handleGetHead(resp, false)

	case http.MethodGet:
		s.handleGetHead(resp, true)

	case http.MethodPost:
		if !strings.HasPrefix(req.RequestURI, svcUriPrefix) || len(req.RequestURI) <= svcUriPrefixLen+1 {
			sendInvalidReq()
			break
		}
		op := req.RequestURI[svcUriPrefixLen:]

		s.lock.Lock()
		defer s.lock.Unlock()

		var err error

		switch op {
		case opStart:
			err = s.ociCtl.Start()
		case opStop:
			err = s.ociCtl.Stop()
		case opRestart:
			err = s.ociCtl.Restart()
		case opEnable:
			err = s.ociCtl.Enable()
		case opDisable:
			err = s.ociCtl.Disable()
		case "drain":
			err = DrainPxVolumeConsumerPods(s.node, true)
		case "drain-managed":
			err = DrainPxVolumeConsumerPods(s.node, false)
		default:
			sendInvalidReq()
			break
		}

		if err == nil {
			msg := fmt.Sprintf("REST action %s completed successfully\n", strings.ToUpper(op))
			logrus.Info(msg)
			content := []byte(msg)
			header := resp.Header()
			header.Add(httpHeaderContentType, "text/plain")
			header.Add(httpHeaderContentLen, strconv.Itoa(len(content)))
			resp.WriteHeader(http.StatusOK)
			resp.Write(content)
		} else {
			logrus.WithError(err).Errorf("Error with REST call POST %s", req.RequestURI)
			http.Error(resp, "INTERNAL ERROR - please check servers logs", http.StatusInternalServerError)
		}

	default:
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
		if s.errorsGrace != nil && time.Now().Before(*s.errorsGrace) {
			logrus.WithError(err).Debug("Could not retrieve PX node status")
		} else { // grace period expired -- warnings become errors
			s.errorsGrace = nil
			logrus.WithError(err).Warn("Could not retrieve PX node status")
		}
		sendResp(http.StatusServiceUnavailable, unknownStatusMsg)
		if pxResp != nil && pxResp.Body != nil {
			// Draining the invalid HTTP PX response, so we can reuse keepalive session
			io.Copy(ioutil.Discard, pxResp.Body)
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
func (s *OciRESTServlet) Start(addr string) {
	if addr == "" {
		addr = defaultOciEndpoint
	}
	if s.srv == nil {
		http.HandleFunc("/", s.handleOciRest)
		s.srv = &http.Server{Addr: addr}
		go func() {
			if err := s.srv.ListenAndServe(); err != nil {
				logrus.WithError(err).Error("Could not start new HTTP server")
			}
		}()
	}
	// CHECKME is s.srv.Shutdown(nil) required?
}
