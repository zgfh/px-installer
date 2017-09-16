package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gorilla/schema"
)

const (
	currentPxImage = "portworx/px-enterprise:1.2.10"
)

type Params struct {
	Cluster     string `schema:"cluster"`
	Kvdb        string `schema:"kvdb"`
	Type        string `schema:"type"`
	Drives      string `schema:"drives"`
	DIface      string `schema:"diface"`
	MIface      string `schema:"miface"`
	Coreos      string `schema:"coreos"`
	Master      string `schema:"master"`
	ZeroStorage string `schema:"zeroStorage"`
	Force       string `schema:"force"`
	EtcdPasswd  string `schema:"etcdPasswd"`
	EtcdCa      string `schema:"etcdCa"`
	EtcdCert    string `schema:"etcdCert"`
	EtcdKey     string `schema:"etcdKey"`
	Acltoken    string `schema:"acltoken"`
	Token       string `schema:"token"`
	Env         string `schema:"env"`
	Openshift   string `schema:"openshift"`
	PxImage     string `schema:"pximage"`
	MasterLess  bool   `schema:"-"`
	IsRunC      bool   `schema:"-"`
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

	p.IsRunC = (p.Type == "runc")
	p.MasterLess = (p.Master != "true")

	if p.PxImage == "" {
		// TODO: Change image for RunC
		p.PxImage = currentPxImage
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

	if !parseStrict {
		// skip unknown keys, unless strict parsing
		decoder.IgnoreUnknownKeys(true)
	}

	err = decoder.Decode(config, r.Form)
	if err != nil {
		return nil, fmt.Errorf("Could not decode form: %s", err)
	}
	fmt.Printf("FROM %v PARSED %+v\n", r.RemoteAddr, config)
	return config, nil
}

// sendError sends back the "400 BAD REQUEST" to the client
func sendError(err error, w http.ResponseWriter) {
	if err == nil {
		err = fmt.Errorf("Unspecified error")
	}
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(err.Error()))
}

// sendUsage sends a simple HTML usage-file back to the browser
func sendUsage(w http.ResponseWriter) {
	cwd, _ := os.Getwd()
	f, err := os.Open(filepath.Join(cwd, "usage.html"))
	if err != nil {
		sendError(err, w)
		return
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	if err != nil {
		sendError(err, w)
	}
}

func main() {
	parseStrict := len(os.Args) > 1 && os.Args[1] == "-strict"

	http.HandleFunc("/kube1.5", func(w http.ResponseWriter, r *http.Request) {
		p, err := parseRequest(r, parseStrict)
		if err != nil {
			sendError(err, w)
			return
		}

		p.Master = ""
		template := "k8s-flexvol-pxd-spec-response.gtpl"
		if len(p.ZeroStorage) != 0 {
			template = "k8s-flexvol-master-worker-response.gtpl"
		}

		content, err := generate(template, p)
		if err != nil {
			sendError(err, w)
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
			sendError(err, w)
			return
		}

		template := "k8s-pxd-spec-response.gtpl"
		if len(p.ZeroStorage) != 0 && (len(p.Master) == 0 || p.Master == "true") {
			template = "k8s-master-worker-response.gtpl"
		}

		content, err := generate(template, p)
		if err != nil {
			sendError(err, w)
			return
		}
		fmt.Fprintf(w, content)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
