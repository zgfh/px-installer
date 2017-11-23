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

	"github.com/gorilla/schema"
)

const (
	templateVersion = "v2"
	// pxImagePrefix will be combined w/ PXTAG to create the linked docker-image
	pxImagePrefix  = "portworx/px-enterprise"
	ociImagePrefix = "portworx/oci-monitor"
	defaultPXTAG   = "1.2.11.4"
)

var (
	// PXTAG is externally defined image tag (can use `go build -ldflags "-X main.PXTAG=1.2.3" ... `
	// to set portworx/px-enterprise:1.2.3)
	PXTAG string
)

// Params contains all parameters passed to us via HTTP.
type Params struct {
	Type        string `schema:"type"   deprecated:"installType"`
	Cluster     string `schema:"c"      deprecated:"cluster"`
	Kvdb        string `schema:"k"      deprecated:"kvdb"`
	Drives      string `schema:"s"      deprecated:"drives"`
	DIface      string `schema:"d"      deprecated:"diface"`
	MIface      string `schema:"m"      deprecated:"miface"`
	KubeVer     string `schema:"kbver"  deprecated:"k8sVersion"`
	Coreos      string `schema:"coreos" deprecated:"coreos"`
	Master      string `schema:"mas"    deprecated:"master"`
	ZeroStorage string `schema:"z"      deprecated:"zeroStorage"`
	Force       string `schema:"f"      deprecated:"force"`
	EtcdPasswd  string `schema:"pwd"    deprecated:"etcdPasswd"`
	EtcdCa      string `schema:"ca"     deprecated:"etcdCa"`
	EtcdCert    string `schema:"cert"   deprecated:"etcdCert"`
	EtcdKey     string `schema:"key"    deprecated:"etcdKey"`
	Acltoken    string `schema:"acl"    deprecated:"acltoken"`
	Token       string `schema:"t"      deprecated:"token"`
	Env         string `schema:"e"      deprecated:"env"`
	Openshift   string `schema:"osft"   deprecated:"openshift"`
	PxImage     string `schema:"px"     deprecated:"pximage"`
	SecretType  string `schema:"st"     deprecated:"secretType"`
	MasterLess  bool   `schema:"-"      deprecated:"-"`
	IsRunC      bool   `schema:"-"      deprecated:"-"`
	TmplVer     string `schema:"-"      deprecated:"-"`
}

func generate(templateFile string, p *Params) (string, error) {

	cwd, _ := os.Getwd()
	t, err := template.ParseFiles(filepath.Join(cwd, templateFile))
	if err != nil {
		return "", err
	}

	// Fix drives entry
	if len(p.Drives) != 0 {
		var b bytes.Buffer
		sep := ""
		for _, dev := range strings.Split(p.Drives, ",") {
			dev = strings.Trim(dev, " ")
			b.WriteString(sep)
			b.WriteString(`"-s", "`)
			b.WriteString(dev)
			b.WriteByte('"')
			sep = ", "
		}
		p.Drives = b.String()
	} else {
		if len(p.Force) != 0 {
			p.Drives = `"-A", "-f"`
		} else {
			p.Drives = `"-a", "-f"`
		}
	}

	// Pre-format Environment entry
	if len(p.Env) != 0 {
		if len(p.Env) != 0 {
			var b bytes.Buffer
			prefix := ""
			for _, e := range strings.Split(p.Env, ",") {
				e = strings.Trim(e, " ")
				entry := strings.SplitN(e, "=", 2)
				if len(entry) == 2 {
					b.WriteString(prefix)
					prefix = "            "
					b.WriteString(`- name: "`)
					b.WriteString(entry[0])
					b.WriteString("\"\n")
					b.WriteString(`              value: "`)
					b.WriteString(entry[1])
					b.WriteString("\"\n")
				}
			}
			p.Env = b.String()
		}
	}

	p.IsRunC = (p.Type == "runc" || p.Type == "oci")
	p.MasterLess = (p.Master != "true")
	p.TmplVer = templateVersion

	// select PX-Image
	if p.PxImage == "" {
		if p.IsRunC {
			p.PxImage = ociImagePrefix + ":" + PXTAG
		} else {
			p.PxImage = pxImagePrefix + ":" + PXTAG
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
func parseRequest(r *http.Request, parseStrict bool) (*Params, error) {
	err := r.ParseForm()
	if err != nil {
		return nil, fmt.Errorf("Could not parse form: %s", err)
	}

	config := new(Params)
	decoder := schema.NewDecoder()

	if q := r.URL.Query(); q != nil && "" != q.Get("cluster") && "" != q.Get("kvdb") {
		log.Println("WARNING: Found 'cluster' and 'kvdb' in query strings - switching to Deprecated parsing")
		decoder.SetAliasTag(`deprecated`)
	}

	if !parseStrict {
		// skip unknown keys, unless strict parsing
		decoder.IgnoreUnknownKeys(true)
	}

	err = decoder.Decode(config, r.Form)
	if err != nil {
		return nil, fmt.Errorf("Could not decode form: %s", err)
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
	log.Printf("ERROR: %s", e)
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

	if PXTAG == "" {
		PXTAG = defaultPXTAG
	}

	http.HandleFunc("/kube1.5", func(w http.ResponseWriter, r *http.Request) {
		p, err := parseRequest(r, parseStrict)
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

		p, err := parseRequest(r, parseStrict)
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
