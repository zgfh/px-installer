package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/gorilla/schema"
)

const (
	vA1             = "v1alpha1"
	vB1             = "v1beta1"
	templateVersion = "v2"
	httpProtocolHdr = "X-Forwarded-Proto"
	// pxImagePrefix will be combined w/ PXTAG to create the linked docker-image
	pxImagePrefix        = "portworx/px-enterprise"
	ociImagePrefix       = "portworx/oci-monitor"
	defaultTalismanImage = "portworx/talisman"
	defaultTalismanTag   = "latest"
	defaultPXTAG         = "1.2.16"
	defaultOCIMonTag     = "1.3.0-rc4"
)

var (
	// PXTAG is externally defined image tag (can use `go build -ldflags "-X main.PXTAG=1.2.3" ... `
	// to set portworx/px-enterprise:1.2.3)
	PXTAG string
	// kbVerRegex matches "1.7.9+coreos.0", "1.7.6+a08f5eeb62", "v1.7.6+a08f5eeb62", "1.7.6", "v1.6.11-gke.0"
	kbVerRegex = regexp.MustCompile(`^[v\s]*(\d+\.\d+\.\d+)(.*)*`)
)

// InstallParams contains all parameters passed to us via HTTP.
type InstallParams struct {
	Type           string `schema:"type"   deprecated:"installType"`
	Cluster        string `schema:"c"      deprecated:"cluster"`
	Kvdb           string `schema:"k"      deprecated:"kvdb"`
	Drives         string `schema:"s"      deprecated:"drives"`
	DIface         string `schema:"d"      deprecated:"diface"`
	MIface         string `schema:"m"      deprecated:"miface"`
	KubeVer        string `schema:"kbver"  deprecated:"k8sVersion"`
	Coreos         string `schema:"coreos" deprecated:"coreos"`
	Master         string `schema:"mas"    deprecated:"master"`
	ZeroStorage    string `schema:"z"      deprecated:"zeroStorage"`
	Force          string `schema:"f"      deprecated:"force"`
	EtcdPasswd     string `schema:"pwd"    deprecated:"etcdPasswd"`
	EtcdCa         string `schema:"ca"     deprecated:"etcdCa"`
	EtcdCert       string `schema:"cert"   deprecated:"etcdCert"`
	EtcdKey        string `schema:"key"    deprecated:"etcdKey"`
	Acltoken       string `schema:"acl"    deprecated:"acltoken"`
	Token          string `schema:"t"      deprecated:"token"`
	Env            string `schema:"e"      deprecated:"env"`
	Openshift      string `schema:"osft"   deprecated:"openshift"`
	PxImage        string `schema:"px"     deprecated:"pximage"`
	SecretType     string `schema:"st"     deprecated:"secretType"`
	JournalDev     string `schema:"j"      deprecated:"journalDev"`
	MasterLess     bool   `schema:"-"      deprecated:"-"`
	IsRunC         bool   `schema:"-"      deprecated:"-"`
	TmplVer        string `schema:"-"      deprecated:"-"`
	Origin         string `schema:"-"      deprecated:"-"`
	RbacAuthVer    string `schema:"-"      deprecated:"-"`
	NeedController bool   `schema:"-"      deprecated:"-"`
	StartStork     bool   `schema:"stork"  deprecated:"stork"`
}

// UpgradeParams contains all parameters passed via HTTP for talisman spec
type UpgradeParams struct {
	OCIMonImage   string `schema:"ociMonImage"`
	OCIMonTag     string `schema:"ociMonTag"`
	PXImage       string `schema:"pxImage"`
	PXTag         string `schema:"pxTag"`
	TalismanImage string `schema:"talismanImage"`
	TalismanTag   string `schema:"talismanTag"`
	KubeVer       string `schema:"kbver"         deprecated:"k8sVersion"`
	RbacAuthVer   string `schema:"-"             deprecated:"-"`
}

func splitCsv(in string) ([]string, error) {
	r := csv.NewReader(strings.NewReader(in))
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil || len(records) < 1 {
		return []string{}, err
	} else if len(records) > 1 {
		return []string{}, fmt.Errorf("Multiline CSV not supported")
	}
	return records[0], err
}

func generateUpgradeSpec(templateFile string, p *UpgradeParams) (string, error) {
	cwd, _ := os.Getwd()
	t, err := template.ParseFiles(filepath.Join(cwd, templateFile))
	if err != nil {
		return "", err
	}

	if len(p.TalismanImage) == 0 {
		p.TalismanImage = defaultTalismanImage
	}

	if len(p.TalismanTag) == 0 {
		p.TalismanTag = defaultTalismanTag
	}

	if len(p.OCIMonTag) == 0 {
		p.OCIMonTag = defaultOCIMonTag
	}

	p.KubeVer, p.RbacAuthVer, _, err = parseKubeVer(p.KubeVer)
	if err != nil {
		return "", err
	}

	var result bytes.Buffer
	err = t.Execute(&result, p)
	if err != nil {
		return "", err
	}

	return result.String(), nil
}

func generateInstallSpec(templateFile string, p *InstallParams) (string, error) {
	cwd, _ := os.Getwd()
	t, err := template.ParseFiles(filepath.Join(cwd, templateFile))
	if err != nil {
		return "", err
	}

	// Fix drives entry
	if len(p.Drives) != 0 {
		devList, err := splitCsv(p.Drives)
		if err != nil {
			return "", err
		}

		var b bytes.Buffer
		sep := ""
		for _, dev := range devList {
			if dev = strings.Trim(dev, ` "`); dev != "" {
				b.WriteString(sep)
				b.WriteString(`"-s", "`)
				b.WriteString(dev)
				b.WriteByte('"')
				sep = ", "
			}
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
		envList, err := splitCsv(p.Env)
		if err != nil {
			return "", err
		}

		var b bytes.Buffer
		prefix := ""
		for _, e := range envList {
			entry := strings.SplitN(e, "=", 2)
			if len(entry) == 2 {
				b.WriteString(prefix)
				prefix = "            "
				b.WriteString(`- name: "`)
				b.WriteString(strings.Trim(entry[0], ` "`))
				b.WriteString("\"\n")
				b.WriteString(`              value: "`)
				b.WriteString(strings.Trim(entry[1], ` "`))
				b.WriteString("\"\n")
			}
		}
		p.Env = b.String()
	}

	p.IsRunC = !strings.HasPrefix(p.Type, "dock") // runC by default, unless dock*
	p.MasterLess = (p.Master != "true")
	p.TmplVer = templateVersion
	p.NeedController = (p.Openshift == "true")
	isGKE := false

	p.KubeVer, p.RbacAuthVer, isGKE, err = parseKubeVer(p.KubeVer)
	if err != nil {
		return "", err
	}

	// GKE (Google Container Engine) extensions - turn on the PVC-Controller
	if isGKE {
		p.NeedController = true
	}

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

func parseKubeVer(ver string) (string, string, bool, error) {
	ver = strings.TrimSpace(ver)
	rbacAuthVer := ""
	isGKE := false

	if len(ver) > 1 { // parse the actual k8s version stripping out unnecessary parts
		matches := kbVerRegex.FindStringSubmatch(ver)
		if len(matches) > 1 {
			ver = matches[1]
			isGKE = strings.HasPrefix(matches[2], "-gke.")
		} else {
			return "", "", false, fmt.Errorf("Invalid Kubernetes version %q."+
				"Please resubmit with a valid kubernetes version (e.g 1.7.8, 1.8.3)", ver)
		}
	}

	// Fix up RbacAuthZ version.
	// * [1.8 docs] https://kubernetes.io/docs/admin/authorization/rbac "As of 1.8, RBAC mode is stable and backed by the rbac.authorization.k8s.io/v1 API"
	// * [1.7 docs] https://v1-7.docs.kubernetes.io/docs/admin/authorization/rbac "As of 1.7 RBAC mode is in beta"
	// * [1.6 docs] https://v1-6.docs.kubernetes.io/docs/admin/authorization/rbac "As of 1.6 RBAC mode is in alpha"
	if ver == "" || strings.HasPrefix(ver, "1.7.") {
		// current Kubernetes default is v1.7.x
		rbacAuthVer = vB1
	} else if ver < "1.7." {
		rbacAuthVer = vA1
	} else {
		rbacAuthVer = "v1"
	}

	// GKE (Google Container Engine) extensions - override v1alphav1 AuthZ which doesn't work on GKE
	if isGKE {
		if rbacAuthVer == vA1 {
			rbacAuthVer = vB1
		}
	}
	return ver, rbacAuthVer, isGKE, nil
}

// parseInstallRequest uses Gorilla schema to process parameters (see http://www.gorillatoolkit.org/pkg/schema)
func parseInstallRequest(r *http.Request, parseStrict bool) (*InstallParams, error) {
	err := r.ParseForm()
	if err != nil {
		return nil, fmt.Errorf("Could not parse form: %s", err)
	}

	config := new(InstallParams)
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

	// Enable stork by default
	if q := r.URL.Query(); q.Get("stork") == "" {
		config.StartStork = true
	}

	log.Printf("FROM %v PARSED %+v\n", r.RemoteAddr, config)

	return config, nil
}

func parseUpgradeRequest(r *http.Request, parseStrict bool) (*UpgradeParams, error) {
	err := r.ParseForm()
	if err != nil {
		return nil, fmt.Errorf("Could not parse form: %s", err)
	}

	config := new(UpgradeParams)
	decoder := schema.NewDecoder()

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

// sendInstallUsage sends a simple HTML usage-file back to the browser
func sendInstallUsage(w http.ResponseWriter) {
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

	http.HandleFunc("/upgrade", func(w http.ResponseWriter, r *http.Request) {
		p, err := parseUpgradeRequest(r, parseStrict)
		if err != nil {
			sendError(http.StatusBadRequest, err, w)
			return
		}

		template := "k8s-px-upgrade.gtpl"
		content, err := generateUpgradeSpec(template, p)
		if err != nil {
			sendError(http.StatusBadRequest, err, w)
			return
		}
		fmt.Fprintf(w, content)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// anything to parse?
		if r.ContentLength == 0 && len(r.URL.RawQuery) == 0 {
			sendInstallUsage(w)
			return
		}

		p, err := parseInstallRequest(r, parseStrict)
		if err != nil {
			sendError(http.StatusBadRequest, err, w)
			return
		}

		// Populate origin, so we can leave it as comment in templates
		p.Origin = "unknown"
		if r.Host != "" && r.URL != nil {
			proto := r.Header.Get(httpProtocolHdr)
			if proto == "" {
				proto = "http"
			}
			p.Origin = fmt.Sprintf("%s://%s%s", proto, r.Host, r.URL)
		}
		log.Printf("Client %q - REQ %s from Referer %q", r.RemoteAddr, p.Origin, r.Referer())
		p.Origin = strings.Replace(p.Origin, "%", "%%", -1)

		template := "k8s-pxd-spec-response.gtpl"
		if len(p.ZeroStorage) != 0 && (len(p.Master) == 0 || p.Master == "true") {
			template = "k8s-master-worker-response.gtpl"
		}

		content, err := generateInstallSpec(template, p)
		if err != nil {
			sendError(http.StatusBadRequest, err, w)
			return
		}
		fmt.Fprintf(w, content)
	})

	log.Printf("Serving at 0.0.0.0:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
