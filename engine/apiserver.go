package engine

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/funkygao/dbus"
	log "github.com/funkygao/log4go"
	"github.com/gorilla/mux"
)

type APIHandler func(w http.ResponseWriter, req *http.Request, params map[string]interface{}) (interface{}, error)

func (this *Engine) launchHttpServ() {
	this.httpRouter = mux.NewRouter()
	this.httpServer = &http.Server{
		Addr:    this.String("apisvr_addr", "127.0.0.1:9876"),
		Handler: this.httpRouter,
	}

	this.setupAPIRoutings()

	var err error
	if this.httpListener, err = net.Listen("tcp", this.httpServer.Addr); err != nil {
		panic(err)
	}

	go this.httpServer.Serve(this.httpListener)
	log.Info("API server ready on http://%s", this.httpServer.Addr)
}

func (this *Engine) stopHttpServ() {
	if this.httpListener != nil {
		this.httpListener.Close()
		log.Info("API server stopped")
	}
}

func (this *Engine) setupAPIRoutings() {
	this.RegisterAPI("/stat", this.httpStat).Methods("GET")
	this.RegisterAPI("/plugins", this.httpPlugins).Methods("GET")
}

func (this *Engine) httpPlugins(w http.ResponseWriter, req *http.Request, params map[string]interface{}) (interface{}, error) {
	names := make([]string, 0, 20)
	for _, pr := range this.InputRunners {
		names = append(names, pr.Name())
	}
	for _, pr := range this.FilterRunners {
		names = append(names, pr.Name())
	}
	for _, pr := range this.OutputRunners {
		names = append(names, pr.Name())
	}

	return names, nil
}

func (this *Engine) httpStat(w http.ResponseWriter, req *http.Request, params map[string]interface{}) (interface{}, error) {
	var output = make(map[string]interface{})
	output["ver"] = dbus.Version
	output["started"] = Globals().StartedAt
	output["elapsed"] = time.Since(Globals().StartedAt).String()
	output["pid"] = this.pid
	output["hostname"] = this.hostname
	output["build"] = dbus.BuildID
	return output, nil
}

func (this *Engine) RegisterAPI(path string, handlerFunc APIHandler) *mux.Route {
	wrappedFunc := func(w http.ResponseWriter, req *http.Request) {
		var (
			ret interface{}
			t1  = time.Now()
		)

		params, err := this.decodeHttpParams(w, req)
		if err == nil {
			ret, err = handlerFunc(w, req, params)
		}

		if err != nil {
			ret = map[string]interface{}{"error": err.Error()}
		}

		w.Header().Set("Server", "dbus")
		w.Header().Set("Content-Type", "application/json")
		var status int
		if err == nil {
			status = http.StatusOK
		} else {
			status = http.StatusInternalServerError
		}
		w.WriteHeader(status)

		// access log
		log.Trace("%s \"%s %s %s\" %d %s",
			req.RemoteAddr,
			req.Method,
			req.RequestURI,
			req.Proto,
			status,
			time.Since(t1))
		if status != http.StatusOK {
			log.Error("ERROR %v", err)
		}

		if ret != nil {
			// pretty write json result
			pretty, _ := json.MarshalIndent(ret, "", "    ")
			w.Write(pretty)
			w.Write([]byte("\n"))
		}
	}

	// path can't be duplicated
	for _, p := range this.httpPaths {
		if p == path {
			panic(path + " already registered")
		}
	}

	this.httpPaths = append(this.httpPaths, path)
	return this.httpRouter.HandleFunc(path, wrappedFunc)
}

func (this *Engine) decodeHttpParams(w http.ResponseWriter, req *http.Request) (map[string]interface{}, error) {
	params := make(map[string]interface{})
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&params)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return params, nil
}
