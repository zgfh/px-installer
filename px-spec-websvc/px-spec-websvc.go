package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/schema"
)

const (
	currentPxImage = "portworx/px-enterprise:1.2.10"
	// TODO: Change to official image for PX-RunC (once released)
	currentRunCImage = "portworx/oci-monitor:latest"
)

var emptyParams = Params{MasterLess: false, IsRunC: false}

type Params struct {
	Cluster     string `schema:"c"    deprecated:"cluster"`
	Kvdb        string `schema:"k"    deprecated:"kvdb"`
	Type        string `schema:"typ"  deprecated:"type"`
	Drives      string `schema:"s"    deprecated:"drives"`
	DIface      string `schema:"d"    deprecated:"diface"`
	MIface      string `schema:"m"    deprecated:"miface"`
	Coreos      string `schema:"co"   deprecated:"coreos"`
	Master      string `schema:"m"    deprecated:"master"`
	ZeroStorage string `schema:"z"    deprecated:"zeroStorage"`
	Force       string `schema:"f"    deprecated:"force"`
	EtcdPasswd  string `schema:"pwd"  deprecated:"etcdPasswd"`
	EtcdCa      string `schema:"ca"   deprecated:"etcdCa"`
	EtcdCert    string `schema:"cert" deprecated:"etcdCert"`
	EtcdKey     string `schema:"key"  deprecated:"etcdKey"`
	Acltoken    string `schema:"acl"  deprecated:"acltoken"`
	Token       string `schema:"t"    deprecated:"token"`
	Env         string `schema:"e"    deprecated:"env"`
	Openshift   string `schema:"osft" deprecated:"openshift"`
	PxImage     string `schema:"px"   deprecated:"pximage"`
	MasterLess  bool   `schema:"-"    deprecated:"-"`
	IsRunC      bool   `schema:"-"    deprecated:"-"`
}

func generate(templateFile string, p *Params) (string, error) {

	cwd, _ := os.Getwd()
	t, err := template.ParseFiles(filepath.Join(cwd, templateFile))
	if err != nil {
		return "", err
	}

	// Fix drives entry
	p.Drives = strings.Trim(p.Drives, " ")
	if len(p.Drives) != 0 {
		var drivesParam bytes.Buffer
		sep := ""
		for _, dev := range strings.Split(p.Drives, ",") {
			drivesParam.WriteString(sep)
			drivesParam.WriteString(`"-s", "`)
			drivesParam.WriteString(dev)
			drivesParam.WriteByte('"')
			sep = ", "
		}
		p.Drives = drivesParam.String()
	} else {
		if len(p.Force) != 0 {
			p.Drives = `"-A", "-f"`
		} else {
			p.Drives = `"-a", "-f"`
		}
	}

	// Pre-format Environment entry
	if len(p.Env) != 0 {
		p.Env = strings.Trim(p.Env, " ")
		if len(p.Env) != 0 {
			var envParam = "env:\n"
			for _, e := range strings.Split(p.Env, ",") {
				entry := strings.SplitN(e, "=", 2)
				if len(entry) == 2 {
					key := entry[0]
					val := entry[1]
					envParam = envParam + "           - name: " + key + "\n"
					envParam = envParam + "             value: " + val + "\n"
				}
			}
			p.Env = envParam
		}
	}

	p.IsRunC = (p.Type == "runc" || p.Type == "oci")
	p.MasterLess = (p.Master != "true")

	if p.PxImage == "" {
		p.PxImage = currentPxImage
		if p.IsRunC {
			p.PxImage = currentRunCImage
		}
	}

	var result bytes.Buffer
	err = t.Execute(&result, p)
	if err != nil {
		return "", err
	}

	return result.String(), nil
}

// parseRequest uses Gorilla schema to process parameters (see http://www.gorillatoolkit.org/pkg/schema)
func parseRequest(r *http.Request, parseStrict, parseDeprecated bool) (*Params, error) {
	err := r.ParseForm()
	if err != nil {
		return nil, fmt.Errorf("Could not parse form: %s", err)
	}

	config := new(Params)
	decoder := schema.NewDecoder()

	if !parseStrict {
		// skip unknown keys, unless strict parsing
		decoder.IgnoreUnknownKeys(true)
	}

	err = decoder.Decode(config, r.Form)
	if err != nil {
		return nil, fmt.Errorf("Could not decode form: %s", err)
	}

	if config != nil && *config == emptyParams {
		log.Printf("Found no parseable values, parsing deprecated schema")
		decoder = schema.NewDecoder()
		decoder.SetAliasTag(`deprecated`)
		decoder.IgnoreUnknownKeys(!parseStrict)
		err = decoder.Decode(config, r.Form)
		if err != nil {
			return nil, fmt.Errorf("Could not decode deprecated form: %s", err)
		}
	}

	log.Printf("FROM %v PARSED %+v\n", r.RemoteAddr, config)

	return config, nil
}

// sendError sends back the "400 BAD REQUEST" to the client
func sendError(code int, err error, w http.ResponseWriter) {
	e := "Unspecified error"
	if err != nil {
		e = err.Error()
	}
	if code <= 0 {
		code = http.StatusBadRequest
	}
	logrus.Error(e)
	w.WriteHeader(code)
	w.Write([]byte(e))
}

// sendUsage sends a simple HTML usage-file back to the browser
func sendUsage(w http.ResponseWriter) {
	cwd, _ := os.Getwd()
	fname := filepath.Join(cwd, "usage.html")
	st, err := os.Stat(fname)
	if err != nil {
		sendError(http.StatusInternalServerError, fmt.Errorf("Could not retrieve usage: %s", err), w)
		return
	}
	w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))
	w.Header().Set("Content-Type", "text/html")
	f, err := os.Open(fname)
	if err != nil {
		sendError(http.StatusInternalServerError, fmt.Errorf("Could not read usage: %s", err), w)
		return
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	if err != nil {
		sendError(http.StatusInternalServerError, fmt.Errorf("Could not send usage: %s", err), w)
	}
}

func main() {
	parseStrict := len(os.Args) > 1 && os.Args[1] == "-strict"

	http.HandleFunc("/kube1.5", func(w http.ResponseWriter, r *http.Request) {
		p, err := parseRequest(r, parseStrict, true)
		if err != nil {
			sendError(http.StatusBadRequest, err, w)
			return
		}

		p.Master = ""
		template := "k8s-flexvol-pxd-spec-response.gtpl"
		if len(p.ZeroStorage) != 0 {
			template = "k8s-flexvol-master-worker-response.gtpl"
		}

		content, err := generate(template, p)
		if err != nil {
			sendError(http.StatusBadRequest, err, w)
			return
		}
		fmt.Fprintf(w, content)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// anything to parse?
		if r.ContentLength == 0 && len(r.URL.RawQuery) == 0 {
			sendUsage(w)
			return
		}

		p, err := parseRequest(r, parseStrict, true)
		if err != nil {
			sendError(http.StatusBadRequest, err, w)
			return
		}

		template := "k8s-pxd-spec-response.gtpl"
		if len(p.ZeroStorage) != 0 && (len(p.Master) == 0 || p.Master == "true") {
			template = "k8s-master-worker-response.gtpl"
		}

		content, err := generate(template, p)
		if err != nil {
			sendError(http.StatusBadRequest, err, w)
			return
		}
		fmt.Fprintf(w, content)
	})

	log.Printf("Serving at 0.0.0.0:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
