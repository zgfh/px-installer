package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"text/template"
)

func generate(etcd string, cluster string) string {

	cwd, _ := os.Getwd()
	p := filepath.Join(cwd, "response.gtpl")

	t, err := template.ParseFiles(p)

	if err != nil {
		log.Println(err)
	}

	data := map[string]string{
		"etcd":    etcd,
		"cluster": cluster,
	}

	var result bytes.Buffer

	err = t.Execute(&result, data)

	if err != nil {
		log.Println(err)
	}

	s := result.String()

	return s
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		etcd := r.URL.Query().Get("etcd")

		cluster := r.URL.Query().Get("cluster")

		fmt.Fprintf(w, generate(etcd, cluster))
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
